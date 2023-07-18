package thread

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"

	"gitee.com/baixudong/gospider/chanx"
)

type DefaultClient = Client[bool]

type Client[T any] struct {
	debug               bool                                    //是否显示调试信息
	threadStartCallBack func(context.Context, int64) (T, error) //每一个线程开始时，根据线程id,创建一个局部对象
	threadEndCallBack   func(context.Context, T) error          //线程被消毁时的回调,再这里可以安全的释放局部对象资源

	ctx2         context.Context      //控制各个协程
	cnl2         context.CancelFunc   //控制各个协程
	ctx          context.Context      //控制主进程，不会关闭各个协程
	cnl          context.CancelFunc   //控制主进程，不会关闭各个协程
	ctx3         context.Context      //chanx 的协程控制
	cnl3         context.CancelFunc   //chanx 的协程控制
	tasks        chan *Task           //任务消费队列
	threadTokens chan struct{}        //线程可用队列
	dones        chan struct{}        //任务完成通知队列
	tasks2       *chanx.Client[*Task] //chanx 的队列任务
	taskCallBack func(*Task) error    //任务回调
	err          error
	maxThreadId  atomic.Int64
	maxNum       int64

	runAfterTime *time.Timer
}

type Task struct {
	Func     any                                //运行的函数
	Args     []any                              //传入的参数
	CallBack func(context.Context, []any) error //回调函数
	Timeout  time.Duration                      //超时时间
	Result   []any                              //函数执行的结果
	Error    error                              //函数错误信息
	ctx      context.Context
	cnl      context.CancelFunc
}

func (obj *Task) Done() <-chan struct{} {
	return obj.ctx.Done()
}

type BaseClientOption[T any] struct {
	Debug               bool                                    //是否显示调试信息
	ThreadStartCallBack func(context.Context, int64) (T, error) //每一个线程开始时，根据线程id,创建一个局部对象
	ThreadEndCallBack   func(context.Context, T) error          //线程被消毁时的回调,再这里可以安全的释放局部对象资源
	TaskCallBack        func(*Task) error                       //有序的任务完成回调
}
type ClientOption = BaseClientOption[bool]

func NewClient(preCtx context.Context, maxNum int64, options ...ClientOption) *DefaultClient {
	return NewBaseClient(preCtx, maxNum, options...)
}

func NewBaseClient[T any](preCtx context.Context, maxNum int64, options ...BaseClientOption[T]) *Client[T] {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	if maxNum < 1 {
		maxNum = 1
	}
	var option BaseClientOption[T]
	if len(options) > 0 {
		option = options[0]
	}
	ctx, cnl := context.WithCancel(preCtx)
	ctx2, cnl2 := context.WithCancel(preCtx)

	tasks := make(chan *Task)
	dones := make(chan struct{})

	threadTokens := make(chan struct{}, maxNum)
	for i := 0; i < int(maxNum); i++ {
		threadTokens <- struct{}{}
	}
	pool := &Client[T]{
		debug:               option.Debug,               //是否显示调试信息
		threadStartCallBack: option.ThreadStartCallBack, //每一个线程开始时，根据线程id,创建一个局部对象
		threadEndCallBack:   option.ThreadEndCallBack,   //线程被消毁时的回调,再这里可以安全的释放局部对象资源
		taskCallBack:        option.TaskCallBack,        //任务回调

		maxNum:       maxNum,
		ctx2:         ctx2,
		cnl2:         cnl2, //关闭协程
		ctx:          ctx,
		cnl:          cnl,   //通知关闭
		tasks:        tasks, //任务队列
		threadTokens: threadTokens,
		dones:        dones,

		runAfterTime: time.NewTimer(0),
	}
	if option.TaskCallBack != nil { //任务完成回调
		ctx3, cnl3 := context.WithCancel(preCtx)
		pool.tasks2 = chanx.NewClient[*Task](preCtx)
		pool.ctx3 = ctx3
		pool.cnl3 = cnl3
		go pool.taskCallBackMain()
	}
	return pool
}
func (obj *Client[T]) taskCallBackMain() {
	defer obj.cnl3()
	defer obj.Close()
	defer obj.tasks2.Close()
	for task := range obj.tasks2.Chan() {
		select {
		case <-obj.ctx2.Done(): //接到关闭线程通知
			obj.err = obj.ctx2.Err()
		case <-task.Done():
			if task.Error != nil { //任务报错，线程报错
				obj.err = task.Error
			}
		}
		if obj.err != nil { //任务报错，开始关闭线程
			return
		}
		if err := obj.taskCallBack(task); err != nil { //任务回调报错，关闭线程
			obj.err = err
			return
		}
	}
}
func (obj *Client[T]) runMain() {
	defer func() {
		if err := recover(); err != nil {
			if obj.err == nil {
				obj.err = err.(error)
			}
			obj.Close()
		}
	}()
	defer func() {
		select {
		case obj.threadTokens <- struct{}{}: //通知有一个协程空闲
		default:
		}
		select {
		case obj.dones <- struct{}{}: //通知协程结束
		default:
		}
	}()
	var runVal T
	var err error
	threadId := obj.maxThreadId.Add(1)  //获取线程id
	if obj.threadStartCallBack != nil { //线程开始回调
		runVal, err = obj.threadStartCallBack(obj.ctx, threadId)
		if err != nil {
			return
		}
	}
	if obj.threadEndCallBack != nil { //处理回调
		defer obj.threadEndCallBack(obj.ctx, runVal)
	}
	for {
		obj.runAfterTime.Reset(time.Second * 30)
		select {
		case <-obj.ctx2.Done(): //通知线程关闭
			return
		case <-obj.ctx.Done(): //通知完成任务后关闭
			select {
			case <-obj.ctx2.Done(): //通知线程关闭
				return
			case task := <-obj.tasks: //接收任务
				obj.run(task, runVal, threadId) //运行任务
			default: //没有任务关闭线程
				return
			}
		case task := <-obj.tasks: //接收任务
			obj.run(task, runVal, threadId)
		case <-obj.runAfterTime.C: //等待线程超时
			return
		}
	}
}

var ErrPoolClosed = errors.New("pool closed")

func (obj *Client[T]) verify(fun any, args []any) error {
	if fun == nil {
		return errors.New("not func")
	}
	typeOfFun := reflect.TypeOf(fun)
	index := 1
	if obj.threadStartCallBack != nil {
		index = 2
		var tmpVal T
		if reflect.TypeOf(tmpVal).String() != typeOfFun.In(1).String() {
			return fmt.Errorf("two params not %T", tmpVal)
		}
	}
	if typeOfFun.Kind() != reflect.Func {
		return errors.New("not func")
	}
	if typeOfFun.NumIn() != len(args)+index {
		return errors.New("args num error")
	}
	if typeOfFun.In(0).String() != "context.Context" {
		return errors.New("frist params not context.Context")
	}
	for i := index; i < len(args)+index; i++ {
		if args[i-index] == nil {
			if reflect.Zero(typeOfFun.In(i)).Interface() != args[i-index] {
				return errors.New("args type not equel")
			}
		} else if !reflect.TypeOf(args[i-index]).ConvertibleTo(typeOfFun.In(i)) {
			return errors.New("args type not equel")
		}
	}
	return nil
}

// 创建task
func (obj *Client[T]) Write(task *Task) (*Task, error) {
	task.ctx, task.cnl = context.WithCancel(obj.ctx2)        //设置任务ctx
	if err := obj.verify(task.Func, task.Args); err != nil { //验证参数
		task.Error = err
		task.cnl()
		return task, err
	}
	for {
		select {
		case <-obj.ctx2.Done(): //接到线程关闭通知
			if obj.Err() != nil {
				task.Error = obj.Err()
			} else {
				task.Error = ErrPoolClosed
			}
			task.cnl()
			return task, task.Error
		case <-obj.ctx.Done(): //接到线程关闭通知
			if obj.Err() != nil {
				task.Error = obj.Err()
			} else {
				task.Error = ErrPoolClosed
			}
			task.cnl()
			return task, task.Error
		case obj.tasks <- task:
			if obj.tasks2 != nil {
				if err := obj.tasks2.Add(task); err != nil {
					if obj.Err() != nil {
						return task, obj.Err()
					}
					return task, err
				}
			}
			return task, nil
		case <-obj.threadTokens: //tasks 写不进去，线程池空闲，开启新的协程消费
			go obj.runMain()
		}
	}
}

type myInt int64

var ThreadId myInt = 0

func GetThreadId(ctx context.Context) int64 { //获取线程id，获取失败返回0
	if ctx == nil {
		return 0
	}
	if val := ctx.Value(ThreadId); val != nil {
		if v, ok := val.(int64); ok {
			return v
		}
	}
	return 0
}
func (obj *Client[T]) run(task *Task, option T, threadId int64) {
	defer task.cnl() //函数结束，任务完成
	defer func() {
		if r := recover(); r != nil {
			task.Error = fmt.Errorf("%v", r)
			if obj.debug {
				debug.PrintStack()
			}
		}
	}()
	timeOut := task.Timeout
	if timeOut > 0 {
		task.ctx, task.cnl = context.WithTimeout(task.ctx, timeOut)
	}
	ctx := context.WithValue(task.ctx, ThreadId, threadId) //线程id 值写入ctx
	index := 1
	if obj.threadStartCallBack != nil {
		index = 2
	}
	params := make([]reflect.Value, len(task.Args)+index)
	params[0] = reflect.ValueOf(ctx)
	if obj.threadStartCallBack != nil {
		params[1] = reflect.ValueOf(option)
	}
	for k, param := range task.Args {
		params[k+index] = reflect.ValueOf(param)
	}
	task.Result = []any{}
	for _, rs := range reflect.ValueOf(task.Func).Call(params) { //执行主方法
		task.Result = append(task.Result, rs.Interface())
	}
	if task.CallBack != nil {
		task.Error = task.CallBack(ctx, task.Result) //执行回调方法
	}
}

func (obj *Client[T]) Join() error { //等待所有任务完成，并关闭pool
	defer obj.runAfterTime.Stop()
	obj.cnl()
	if obj.tasks2 != nil {
		obj.tasks2.Join()
		<-obj.ctx3.Done()
	}
	if obj.ThreadSize() <= 0 {
		obj.cnl2()
		return obj.Err()
	}
	for {
		select {
		case <-obj.ctx2.Done(): //线程关闭推出
			return obj.Err()
		case <-obj.dones:
			if obj.ThreadSize() <= 0 {
				obj.cnl2()
				return obj.Err()
			}
		}
	}
}

func (obj *Client[T]) Close() { //告诉所有协程，立即结束任务
	defer obj.runAfterTime.Stop()
	obj.cnl()
	if obj.tasks2 != nil {
		obj.tasks2.Close()
	}
	obj.cnl2()
}
func (obj *Client[T]) Err() error { //错误
	return obj.err
}
func (obj *Client[T]) Done() <-chan struct{} { //所有任务执行完毕
	return obj.ctx2.Done()
}
func (obj *Client[T]) ThreadSize() int64 { //创建的协程数量
	return obj.maxNum - int64(len(obj.threadTokens))
}
func (obj *Client[T]) Empty() bool { //任务是否为空
	if obj.ThreadSize() <= 0 && len(obj.tasks) == 0 {
		return true
	}
	return false
}
