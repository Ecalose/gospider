# 版本
```
go 1.20
```
# 修改指纹
## 添加函数
```go
type Upg struct {
	connPool *http2clientConnPool
	t        *http2Transport
	server   *http2Server
}
type UpgOption struct {
	H2Ja3Spec             ja3.H2Ja3Spec
	DisableCompression    bool
	DialTLSContext        func(ctx context.Context, network string, addr string, cfg *tls.Config) (net.Conn, error)
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	Server                bool //是否为服务端
}

func NewUpg(t1 *http.Transport, options ...UpgOption) *Upg {
	//初始化参数
	var option UpgOption
	if len(options) > 0 {
		option = options[0]
	}
	var headerTableSize uint32 = 65536
	var maxHeaderListSize uint32 = 262144
	var streamFlow uint32 = 6291456

	if option.H2Ja3Spec.InitialSetting != nil {
		for _, setting := range option.H2Ja3Spec.InitialSetting {
			switch setting.Id {
			case 1:
				headerTableSize = setting.Val
			case 6:
				maxHeaderListSize = setting.Val
			case 4:
				streamFlow = setting.Val
			}
		}
	} else {
		option.H2Ja3Spec.InitialSetting = []ja3.Setting{
			{Id: 1, Val: headerTableSize},
			{Id: 2, Val: 0},
			{Id: 3, Val: 1000},
			{Id: 4, Val: 6291456},
			{Id: 6, Val: maxHeaderListSize},
		}
	}
	if !option.H2Ja3Spec.Priority.Exclusive && option.H2Ja3Spec.Priority.StreamDep == 0 && option.H2Ja3Spec.Priority.Weight == 0 {
		option.H2Ja3Spec.Priority = ja3.Priority{
			Exclusive: true,
			StreamDep: 0,
			Weight:    255,
		}
	}
	if option.H2Ja3Spec.OrderHeaders == nil {
		option.H2Ja3Spec.OrderHeaders = []string{":method", ":authority", ":scheme", ":path"}
	}
	if option.H2Ja3Spec.ConnFlow == 0 {
		option.H2Ja3Spec.ConnFlow = 15663105
	}

	if option.IdleConnTimeout == 0 {
		option.IdleConnTimeout = time.Second * 30
	}
	if option.TLSHandshakeTimeout == 0 {
		option.TLSHandshakeTimeout = time.Second * 15
	}
	if option.ResponseHeaderTimeout == 0 {
		option.ResponseHeaderTimeout = time.Second * 30
	}
	//开始创建服务端
	if option.Server {
		server := new(http2Server)
		server.state = &http2serverInternalState{activeConns: make(map[*http2serverConn]struct{})}
		server.IdleTimeout = option.IdleConnTimeout //检测连接是否健康的间隔时间
		return &Upg{
			server: server,
		}
	}
	//开始创建客户端
	connPool := new(http2clientConnPool)
	t2 := &http2Transport{
		ConnPool: http2noDialClientConnPool{connPool},
		t1:       t1,

		h2Ja3Spec:                 option.H2Ja3Spec,
		streamFlow:                streamFlow,
		MaxDecoderHeaderTableSize: headerTableSize,   //1:initialHeaderTableSize,65536
		MaxEncoderHeaderTableSize: headerTableSize,   //1:initialHeaderTableSize,65536
		MaxHeaderListSize:         maxHeaderListSize, //6:MaxHeaderListSize,262144
		DisableCompression:        option.DisableCompression,

		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		DialTLSContext:   option.DialTLSContext,
		ReadIdleTimeout:  option.IdleConnTimeout, //检测连接是否健康的间隔时间
		PingTimeout:      option.TLSHandshakeTimeout,
		WriteByteTimeout: option.ResponseHeaderTimeout,
	}
	connPool.t = t2
	if t1 != nil {
		t1.RegisterProtocol("https", http2noDialH2RoundTripper{t2})
	}
	return &Upg{
		connPool: connPool,
		t:        t2,
	}
}
func (obj *Upg) CloseIdleConnections() {
	if obj.t != nil {
		obj.t.CloseIdleConnections()
	}
}
func (obj *Upg) UpgradeFn(authority string, c net.Conn) http.RoundTripper {
	addr := http2authorityAddr("https", authority)
	if used, err := obj.connPool.addConnIfNeeded(addr, obj.t, c); err != nil {
		defer c.Close()
		return http2erringRoundTripper{err}
	} else if !used {
		defer c.Close()
	}
	return obj.t
}
func (obj *Upg) ServerConn(ctx context.Context, c net.Conn, h http.Handler) {
	obj.server.ServeConn(c, &http2ServeConnOpts{
		Context: ctx,
		Handler: h,
	})
}
```

## 修改标头顺序
### 修改 enumerateHeaders 函数中的请求头顺序: 函数传参f 改为f2 ,函数内部重新定义函数f
```go
	func(f2 func(name, value string)) {
		//开头
		headers := http.Header{}
		f := func(name, value string) {
			headers.Add(name, value)
		}
		//中间代码不变
		//中间代码不变
		//中间代码不变
		//中间代码不变

		//结尾
		ll := kinds.NewSet[string]()
		for _, kk := range cc.t.h2Ja3Spec.OrderHeaders {
			for i := 0; i < 2; i++ {
				if i == 1 {
					kk = strings.Title(kk)
				}
				if vvs, ok := headers[kk]; ok {
					ll.Add(kk)
					for _, vv := range vvs {
						f2(kk, vv)
					}
					break
				}
			}
		}
		for kk, vvs := range headers {
			for _, vv := range vvs {
				if !ll.Has(kk) {
					f2(kk, vv)
				}
			}
		}
	}
```
## 修改streamFlow 值,删除 常量 http2transportDefaultStreamFlow ,并替换为 http2Transport 中的 streamFlow

## 修改 newClientConn 函数中的   initialSettings 和 WriteWindowUpdate
### 删除常量 http2transportDefaultConnFlow ，http2transportDefaultConnFlow  替换为  t.h2Ja3Spec.ConnFlow
```go
	initialSettings := make([]http2Setting, len(t.h2Ja3Spec.InitialSetting))
	for i, setting := range t.h2Ja3Spec.InitialSetting {
		initialSettings[i] = http2Setting{ID: http2SettingID(setting.Id), Val: setting.Val}
	}
	cc.bw.Write(http2clientPreface)
	cc.fr.WriteSettings(initialSettings...)
	cc.fr.WriteWindowUpdate(0, t.h2Ja3Spec.ConnFlow)
	cc.inflow.add(int32(t.h2Ja3Spec.ConnFlow) + http2initialWindowSize)
```
## 修改 ClientConn 的 writeHeaders 函数 的 first==true 时候的WriteHeaders 参数增加 Priority 值
```go
		if first {
			cc.fr.WriteHeaders(HeadersFrameParam{//这里是一个指纹检测点
				StreamID:      streamID,
				BlockFragment: chunk,
				EndStream:     endStream,
				EndHeaders:    endHeaders,
				Priority: http2PriorityParam{
					StreamDep: cc.t.h2Ja3Spec.Priority.StreamDep,
					Exclusive: cc.t.h2Ja3Spec.Priority.Exclusive,
					Weight:    cc.t.h2Ja3Spec.Priority.Weight,
				},
			})
			first = false
		} else {
			cc.fr.WriteContinuation(streamID, endHeaders, chunk)
		}
```
# 添加ctx 以解决http2 代理转发无法及时同步导致的bug
## 修改 http2serverConn 的  serve() 方法中的代码，将下面代码放到for 循环末尾
```go
	for {
			//源代码
			//下面代码放到末尾
		if !sc.inFrameScheduleLoop && !sc.inGoAway && !sc.needToSendGoAway && !sc.needToSendSettingsAck && !sc.needsFrameFlush && !sc.writingFrame {
			if tconn, ok := sc.conn.(interface{ Ctx() context.Context }); ok {
				select {
				case <-tconn.Ctx().Done():
					return
				default:
				}
			}
		}
	}
```

