package tools

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/md5"
	rand2 "crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"image"
	"io"
	"math"
	"math/big"
	"math/rand"
	"os/signal"
	"syscall"

	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/andybalholm/brotli"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/gjson"

	_ "image/png"

	_ "embed"

	"gitee.com/baixudong/gospider/kinds"
	"gitee.com/baixudong/gospider/re"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
)

//go:embed gospider.crt
var CrtFile []byte

//go:embed gospider.key
var KeyFile []byte
var jsonConfig = jsoniter.Config{
	EscapeHTML:    true,
	CaseSensitive: true,
}.Froze()

// 路径是否存在
func PathExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		fmt.Println(err)
		return false
	}
	return true
}

// 创建目录
func MkDir(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	return err
}

// 拼接url
func UrlJoin(base, href string) (string, error) {
	baseUrl, err := url.Parse(base)
	if err != nil {
		return base, err
	}
	refUrl, err := url.Parse(href)
	if err != nil {
		return href, err
	}
	return baseUrl.ResolveReference(refUrl).String(), nil
}

// 网页解码，并返回 编码
func Charset(content []byte, content_type string) ([]byte, string, error) {
	chset, chset_name, _ := charset.DetermineEncoding(content, content_type)
	chset_content, err := chset.NewDecoder().Bytes(content)
	return chset_content, chset_name, err
}

// 转码
func Decode[T string | []byte](txt T, code string) T {
	var result any
	switch val := (any)(txt).(type) {
	case string:
		switch code {
		case "gb2312":
			result, _ = simplifiedchinese.HZGB2312.NewDecoder().String(val)
		case "gbk":
			result, _ = simplifiedchinese.GBK.NewDecoder().String(val)
		default:
			result = val
		}
	case []byte:
		switch code {
		case "gb2312":
			result, _ = simplifiedchinese.HZGB2312.NewDecoder().Bytes(val)
		case "gbk":
			result, _ = simplifiedchinese.GBK.NewDecoder().Bytes(val)
		default:
			result = val
		}
	}
	return result.(T)
}

// 转码
func DecodeRead(txt io.Reader, code string) io.Reader {
	switch code {
	case "gb2312":
		txt = simplifiedchinese.HZGB2312.NewDecoder().Reader(txt)
	case "gbk":
		txt = simplifiedchinese.GBK.NewDecoder().Reader(txt)
	}
	return txt
}

// 转成json
func Any2json(data any) (gjson.Result, error) {
	var result gjson.Result
	switch value := data.(type) {
	case []byte:
		result = gjson.ParseBytes(value)
	case string:
		result = gjson.Parse(value)
	default:
		marstr, err := JsonMarshal(value)
		if err != nil {
			return result, err
		}
		result = gjson.ParseBytes(marstr)
	}
	return result, nil
}

func JsonMarshal(data any) ([]byte, error) {
	return jsonConfig.Marshal(data)
}
func JsonUnMarshal(data []byte, v any) error {
	return jsonConfig.Unmarshal(data, v)
}

// 转成struct
func Any2struct(data any, stru any) error {
	con, err := JsonMarshal(data)
	if err != nil {
		return err
	}
	return JsonUnMarshal(con, stru)
}

// 合并两个结构体 *ci c2
func Merge(c1 any, c2 any) {
	v2 := reflect.ValueOf(c2)             //初始化为c2保管的具体值的v2
	v1_elem := reflect.ValueOf(c1).Elem() //返回 c1 指针保管的值
	for i := 0; i < v2.NumField(); i++ {
		field2 := v2.Field(i)                                                                                             //返回结构体的第i个字段
		if !reflect.DeepEqual(field2.Interface(), reflect.Zero(field2.Type()).Interface()) && v1_elem.Field(i).CanSet() { //如果第二个构造体 这个字段不为空
			v1_elem.Field(i).Set(field2) //设置值
		}
	}
}

var zhNumStr = "[零〇一壹二贰三叁四肆五伍六陆七柒八捌九玖]"

var zhNumMap = map[string]string{
	"零": "0",
	"〇": "0",

	"一": "1",
	"壹": "1",

	"二": "2",
	"贰": "2",

	"三": "3",
	"叁": "3",

	"四": "4",
	"肆": "4",

	"五": "5",
	"伍": "5",

	"六": "6",
	"陆": "6",

	"七": "7",
	"柒": "7",

	"八": "8",
	"捌": "8",

	"九": "9",
	"玖": "9",
}

// 文本解析时间
func GetTime(txt string, desc ...bool) string {
	txt = re.SubFunc(zhNumStr, func(s string) string {
		return zhNumMap[s]
	}, txt)
	txt = re.SubFunc(`\d?十\d*`, func(s string) string {
		if s == "十" {
			return "10"
		} else if strings.HasPrefix(s, "十") {
			return strings.Replace(s, "十", "1", 1)
		} else if strings.HasSuffix(s, "十") {
			return strings.Replace(s, "十", "0", 1)
		} else {
			return strings.Replace(s, "十", "", 1)
		}
	}, txt)

	lls := re.FindAll(`\D(20[012]\d(?:[\.\-/]|\s?年\s?)[01]?\d(?:[\.\-/]|\s?月\s?)[0123]?\d)\D`, "a"+txt+"a")
	data := kinds.NewSet[string]()
	for _, ll := range lls {
		ll_str := re.Sub(`[\.\-年月/]`, "-", ll.Group(1))
		value_lls := strings.Split(ll_str, "-")
		moth := value_lls[1]
		day := value_lls[2]
		if len(moth) == 1 {
			moth = "0" + moth
		}
		if len(day) == 1 {
			day = "0" + day
		}
		data.Add(value_lls[0] + "-" + moth + "-" + day)
	}
	var result string
	if data.Len() > 0 {
		temData := data.Array()
		sort.Strings(temData)
		if len(desc) > 0 && desc[0] {
			result = temData[data.Len()-1]
		} else {
			result = temData[0]
		}
	}
	return result
}

// 路径转义
func PathEscape(txt string) string { //空格转换%20
	return url.PathEscape(txt)
}

// 路径解析
func PathUnescape(txt string) (string, error) {
	return url.PathUnescape(txt)
}

// 参数转义
func QueryEscape(txt string) string { //空格转换为+
	return url.QueryEscape(txt)
}

// 参数解析
func QueryUnescape(txt string) (string, error) {
	return url.QueryUnescape(txt)
}

// 默认目录
func GetDefaultDir() (string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %v", err)
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(userHomeDir, "AppData", "Local"), nil
	case "darwin":
		return filepath.Join(userHomeDir, "Library", "Caches"), nil
	case "linux":
		return filepath.Join(userHomeDir, ".cache"), nil
	}
	return "", errors.New("could not determine cache directory")
}

// 拼接路径
func PathJoin(elem ...string) string {
	return filepath.Join(elem...)
}
func GetHost(addrTypes ...int) net.IP {
	hosts := GetHosts(addrTypes...)
	if len(hosts) == 0 {
		return nil
	} else {
		return hosts[0]
	}
}
func GetHosts(addrTypes ...int) []net.IP {
	var addrType int
	if len(addrTypes) > 0 {
		addrType = addrTypes[0]
	}
	result := []net.IP{}
	lls, err := net.InterfaceAddrs()
	if err != nil {
		return result
	}
	for _, ll := range lls {
		mm, ok := ll.(*net.IPNet)
		if ok && mm.IP.IsPrivate() {
			if addrType == 0 || ParseIp(mm.IP) == addrType {
				result = append(result, mm.IP)
			}
		}
	}
	return result
}

// aes加密
func AesEncode(val []byte, key []byte) (string, error) {
	keyLen := len(key)
	if keyLen > 16 {
		key = key[:16]
	} else if keyLen < 16 {
		key = append(key, make([]byte, 16-keyLen)...)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	padNum := blockSize - len(val)%blockSize
	pad := bytes.Repeat([]byte{byte(padNum)}, padNum)
	val = append(val, pad...)

	blockMode := cipher.NewCBCEncrypter(block, key)
	blockMode.CryptBlocks(val, val)
	return Base64Encode(val), nil
}

// HmacSha1 加密
func HmacSha1[T string | []byte](val, key T) []byte {
	var mac hash.Hash
	switch con := (any)(key).(type) {
	case string:
		mac = hmac.New(sha1.New, StringToBytes(con))
	case []byte:
		mac = hmac.New(sha1.New, con)
	}

	switch con := (any)(val).(type) {
	case string:
		mac.Write(StringToBytes(con))
	case []byte:
		mac.Write(con)
	}
	return mac.Sum(nil)
}

// Sha1 加密
func Sha1[T string | []byte](val T) []byte {
	mac := sha1.New()
	switch con := (any)(val).(type) {
	case string:
		mac.Write(StringToBytes(con))
	case []byte:
		mac.Write(con)
	}
	return mac.Sum(nil)
}

// md5 加密
func Md5[T string | []byte](val T) [16]byte {
	var result [16]byte
	switch con := (any)(val).(type) {
	case string:
		result = md5.Sum(StringToBytes(con))
	case []byte:
		result = md5.Sum(con)
	}
	return result
}

// base64 加密
func Base64Encode[T string | []byte](val T) string {
	switch con := (any)(val).(type) {
	case string:
		return base64.StdEncoding.EncodeToString(StringToBytes(con))
	case []byte:
		return base64.StdEncoding.EncodeToString(con)
	}
	return ""
}

// base64解密
func Base64Decode(val string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(val)
}

func Hex(val any) string {
	return fmt.Sprintf("%x", val)
}

// ase解密
func AesDecode(val string, key []byte) ([]byte, error) {
	src, err := Base64Decode(val)
	if err != nil {
		return nil, nil
	}
	keyLen := len(key)
	if keyLen > 16 {
		key = key[:16]
	} else if keyLen < 16 {
		key = append(key, make([]byte, 16-keyLen)...)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, key)
	blockMode.CryptBlocks(src, src)
	n := len(src)
	unPadNum := int(src[n-1])
	src = src[:n-unPadNum]
	return src, nil
}

type readClose struct {
	reader *brotli.Reader
}

func (obj *readClose) Read(p []byte) (n int, err error) { return obj.reader.Read(p) }
func (obj *readClose) Close() error                     { return obj.reader.Reset(nil) }

// 压缩解码
func CompressionBrDecode(ctx context.Context, r *bytes.Buffer) (*bytes.Buffer, error) {
	rs := bytes.NewBuffer(nil)
	return rs, CopyWitchContext(ctx, rs, &readClose{reader: brotli.NewReader(r)})
}
func CompressionDeflateDecode(ctx context.Context, r *bytes.Buffer) (*bytes.Buffer, error) {
	rs, reader := bytes.NewBuffer(nil), flate.NewReader(r)
	defer reader.Close()
	return rs, CopyWitchContext(ctx, rs, reader)
}
func CompressionGzipDecode(ctx context.Context, r *bytes.Buffer) (*bytes.Buffer, error) {
	reader, err := gzip.NewReader(r)
	if err != nil {
		return r, err
	}
	defer reader.Close()
	rs := bytes.NewBuffer(nil)
	return rs, CopyWitchContext(ctx, rs, reader)
}
func CompressionZlibDecode(ctx context.Context, r *bytes.Buffer) (*bytes.Buffer, error) {
	reader, err := zlib.NewReader(r)
	if err != nil {
		return r, err
	}
	defer reader.Close()
	rs := bytes.NewBuffer(nil)
	return rs, CopyWitchContext(ctx, rs, reader)
}

// 压缩解码
func CompressionDecode(ctx context.Context, r *bytes.Buffer, encoding string) (*bytes.Buffer, error) {
	switch encoding {
	case "br":
		return CompressionBrDecode(ctx, r)
	case "deflate":
		return CompressionDeflateDecode(ctx, r)
	case "gzip":
		return CompressionGzipDecode(ctx, r)
	case "zlib":
		return CompressionZlibDecode(ctx, r)
	default:
		return r, nil
	}
}

// 字节串转字符串
func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

// 字符串转字节串
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// 随机函数
var Rand = rand.New(rand.NewSource(time.Now().UnixMilli()))

var bidChars = "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"

var defaultAlphabet = []rune(bidChars)
var defaultAlphabetLen = len(defaultAlphabet)

// naoid 生成
func NaoId(l ...int) string {
	var size int
	if len(l) > 0 {
		size = l[0]
	} else {
		size = 21
	}
	id := make([]rune, size)
	for i := 0; i < size; i++ {
		id[i] = defaultAlphabet[RanInt(0, defaultAlphabetLen)]
	}
	return string(id)
}

// naoid 生成
func NaoIdWithStr(val string, l ...int) string {
	var size int
	if len(l) > 0 {
		size = l[0]
	} else {
		size = 21
	}
	alphabet := []rune(val)
	alphabetLen := len(alphabet)

	id := make([]rune, size)
	for i := 0; i < size; i++ {
		id[i] = alphabet[RanInt(0, alphabetLen)]
	}
	return string(id)
}

type bidclient struct {
	bidMax       int64
	curNum       int64
	bidPid       string
	bidCharsILen int64
	bidCharsFLen float64
	bidChars     string
	curTime      int64
	lock         sync.Mutex
}

func newBidClient() *bidclient {
	bidCli := &bidclient{
		bidMax:   78074896 - 1,
		bidChars: bidChars,
	}
	bidCli.bidCharsILen = int64(len(bidCli.bidChars))
	bidCli.bidCharsFLen = float64(len(bidCli.bidChars))
	bidCli.bidPid = bidCli.bidEncode(int64(os.Getpid()), 4)
	return bidCli
}

var bidClient = newBidClient()

type BonId struct {
	Timestamp int64
	Count     int64
	String    string
}

func (obj *bidclient) bidEncode(num int64, lens ...int) string {
	bytesResult := []byte{}
	for num > 0 {
		bytesResult = append(bytesResult, obj.bidChars[num%obj.bidCharsILen])
		num = num / obj.bidCharsILen
	}
	for left, right := 0, len(bytesResult)-1; left < right; left, right = left+1, right-1 {
		bytesResult[left], bytesResult[right] = bytesResult[right], bytesResult[left]
	}
	result := BytesToString(bytesResult)
	if len(lens) == 0 {
		return result
	}
	if len(result) < lens[0] {
		res := bytes.NewBuffer(nil)
		for i := len(result); i < lens[0]; i++ {
			res.WriteString("0")
		}
		res.Write(bytesResult)
		return res.String()
	} else {
		return result
	}
}

func (obj *bidclient) bidDecode(str string) (int64, error) {
	var num int64
	n := len(str)
	for i := 0; i < n; i++ {
		pos := strings.IndexByte(obj.bidChars, str[i])
		if pos == -1 {
			return 0, errors.New("char error")
		}
		num += int64(math.Pow(obj.bidCharsFLen, float64(n-i-1)) * float64(pos))
	}
	return num, nil
}

func NewBonId() BonId {
	bidClient.lock.Lock()
	defer bidClient.lock.Unlock()
	var result BonId
	result.Timestamp = time.Now().Unix()
	if bidClient.curTime != result.Timestamp {
		bidClient.curTime = result.Timestamp
		bidClient.curNum = -1
	} else if bidClient.curNum >= bidClient.bidMax {
		panic("too max num")
	}
	bidClient.curNum++
	result.Count = bidClient.curNum
	result.String = bidClient.bidEncode(result.Timestamp, 5) + bidClient.bidEncode(bidClient.curNum, 4) + bidClient.bidPid + NaoId(4)
	return result
}
func BonIdFromString(val string) (BonId, error) {
	var result BonId
	if len(val) != 17 {
		return result, errors.New("错误的字符串")
	}
	tt, err := bidClient.bidDecode(val[:5])
	if err != nil {
		return result, err
	}
	result.Timestamp = tt
	tt, err = bidClient.bidDecode(val[5:9])
	if err != nil {
		return result, err
	}
	result.Count = tt
	result.String = val
	return result, err
}
func FreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, err
}
func RanInt64(val, val2 int64) int64 {
	if val == val2 {
		return val
	} else if val2 > val {
		return val + Rand.Int63n(val2-val)
	} else {
		return val2 + Rand.Int63n(val-val2)
	}
}
func RanInt(val, val2 int) int {
	if val == val2 {
		return val
	} else if val2 > val {
		return val + Rand.Intn(val2-val)
	} else {
		return val2 + Rand.Intn(val-val2)
	}
}
func RanFloat64(val, val2 int64) float64 {
	return float64(RanInt64(val, val2)) + Rand.Float64()
}

// :param point0: 起点
// :param point1: 终点
// :param control_point: 控制点
// :param point_nums: 生成曲线坐标点的数量.数量越多图越凹凸不平，越少越平滑
func GetTrack(point0, point1 [2]float64, point_nums float64) [][2]float64 {
	x1, y1 := point1[0], point1[1]
	abs_x := math.Abs(point0[0]-x1) / 2 //两点横坐标相减绝对值/2
	abs_y := math.Abs(point0[1]-y1) / 2 //两点纵坐标相减绝对值/2
	pointList := [][2]float64{}
	cx, cy := (point0[0]+x1)/2+RanFloat64(int64(abs_x*-1), int64(abs_x)), (point0[1]+y1)/2+RanFloat64(int64(abs_y*-1), int64(abs_y))
	var i float64
	for i = 0; i < point_nums+1; i++ {
		t := i / point_nums
		x := math.Pow(1-t, 2)*point0[0] + 2*t*(1-t)*cx + math.Pow(t, 2)*x1
		y := math.Pow(1-t, 2)*point0[1] + 2*t*(1-t)*cy + math.Pow(t, 2)*y1
		pointList = append(pointList, [2]float64{x, y})
	}
	return pointList
}

func DelSliceIndex[T any](val []T, indexs ...int) []T {
	indexs = kinds.NewSet(indexs...).Array()
	l := len(indexs)
	switch l {
	case 0:
		return val
	case 1:
		return append(val[:indexs[0]], val[indexs[0]+1:]...)
	default:
		sort.Ints(indexs)
		for i := l - 1; i >= 0; i-- {
			val = DelSliceIndex(val, indexs[i])
		}
		return val
	}
}
func WrapError(err error, val ...any) error {
	return fmt.Errorf("%w,%s", err, fmt.Sprint(val...))
}

func CopyWitchContext(ctx context.Context, writer io.Writer, reader io.ReadCloser) (err error) {
	defer func() {
		if err != nil && errors.Is(err, io.ErrUnexpectedEOF) {
			err = nil
		}
	}()
	if ctx == nil {
		defer func() {
			if recErr := recover(); recErr != nil && err == nil {
				err, _ = recErr.(error)
			}
		}()
		_, err = io.Copy(writer, reader)
		return
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	done := make(chan struct{})
	go func() {
		defer func() {
			if recErr := recover(); recErr != nil && err == nil {
				err, _ = recErr.(error)
			}
			close(done)
		}()
		_, err = io.Copy(writer, reader)
	}()
	select {
	case <-ctx.Done():
		reader.Close()
		err = ctx.Err()
	case <-done:
	}
	return
}
func ParseHost(host string) (net.IP, int) {
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return ip4, 4
		} else if ip6 := ip.To16(); ip6 != nil {
			return ip6, 6
		}
	}
	return nil, 0
}
func ParseIp(ip net.IP) int {
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return 4
		} else if ip6 := ip.To16(); ip6 != nil {
			return 6
		}
	}
	return 0
}
func SplitHostPort(address string) (string, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	portnum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	if 1 > portnum || portnum > 0xffff {
		return "", 0, errors.New("port number out of range " + port)
	}
	return host, portnum, nil
}

// 生成根证书
func CreateRootCert(key *ecdsa.PrivateKey) (*x509.Certificate, error) {
	beforDate, err := time.ParseInLocation(time.DateOnly, "2023-03-20", time.Local)
	if err != nil {
		return nil, err
	}
	afterDate, err := time.ParseInLocation(time.DateOnly, "3023-03-20", time.Local)
	if err != nil {
		return nil, err
	}
	rootCsr := &x509.Certificate{
		Version:      3,
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Province:           []string{"Shanghai"},
			Locality:           []string{"Shanghai"},
			Organization:       []string{"GoSpider"},
			OrganizationalUnit: []string{"GoSpiderProxy"},
			CommonName:         "Gospider Root CA",
		},
		NotBefore:             beforDate,
		NotAfter:              afterDate,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	rootDer, err := x509.CreateCertificate(rand2.Reader, rootCsr, rootCsr, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(rootDer)
}

// 生成私钥
func CreateCertKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand2.Reader)
}
func GetCertWithCN(rootCert *x509.Certificate, key *ecdsa.PrivateKey, commonName string) (*x509.Certificate, error) {
	csr := &x509.Certificate{
		Version:               3,
		SerialNumber:          big.NewInt(time.Now().Unix()),
		Subject:               rootCert.Subject,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1000, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	csr.IPAddresses = []net.IP{}
	if commonName != "" {
		if ip, ipType := ParseHost(commonName); ipType == 0 {
			csr.Subject.CommonName = commonName
			csr.DNSNames = []string{commonName}
		} else {
			csr.IPAddresses = append(csr.IPAddresses, ip)
		}
	}
	der, err := x509.CreateCertificate(rand2.Reader, csr, rootCert, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

func CreateCertWithCert(rootCert *x509.Certificate, key *ecdsa.PrivateKey, preCert *x509.Certificate) (*x509.Certificate, error) {
	if preCert.DNSNames == nil && preCert.Subject.CommonName != "" {
		preCert.DNSNames = []string{preCert.Subject.CommonName}
	}
	rootCert.Subject.CommonName = preCert.Subject.CommonName
	csr := &x509.Certificate{
		Version:               3,
		SerialNumber:          big.NewInt(time.Now().Unix()),
		Subject:               rootCert.Subject,
		DNSNames:              preCert.DNSNames,
		IPAddresses:           preCert.IPAddresses,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1000, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	if len(preCert.DNSNames) > 0 {
		csr.Subject.CommonName = preCert.DNSNames[0]
	}
	der, err := x509.CreateCertificate(rand2.Reader, csr, rootCert, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}
func CreateProxyCertWithName(serverName string) (tlsCert tls.Certificate, err error) {
	crt, err := LoadCertData(CrtFile)
	if err != nil {
		return tlsCert, err
	}
	key, err := LoadCertKeyData(KeyFile)
	if err != nil {
		return tlsCert, err
	}
	cert, err := GetCertWithCN(crt, key, serverName)
	if err != nil {
		return tlsCert, err
	}
	return GetTlsCert(cert, key)
}
func CreateProxyCertWithCert(crt *x509.Certificate, key *ecdsa.PrivateKey, preCert *x509.Certificate) (tlsCert tls.Certificate, err error) {
	if crt == nil {
		crt, err = LoadCertData(CrtFile)
		if err != nil {
			return tlsCert, err
		}
	}
	if key == nil {
		key, err = LoadCertKeyData(KeyFile)
		if err != nil {
			return tlsCert, err
		}
	}
	cert, err := CreateCertWithCert(crt, key, preCert)
	if err != nil {
		return tlsCert, err
	}
	return GetTlsCert(cert, key)
}
func GetTlsCert(cert *x509.Certificate, key *ecdsa.PrivateKey) (tls.Certificate, error) {
	keyFile, err := GetCertKeyData(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(GetCertData(cert), keyFile)
}
func GetCertData(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}
func GetCertKeyData(key *ecdsa.PrivateKey) ([]byte, error) {
	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), nil
}
func LoadCertKeyData(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	return x509.ParseECPrivateKey(block.Bytes)
}
func LoadCertData(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	return x509.ParseCertificate(block.Bytes)
}

func GetServerName(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
func CopySlices[T any](value []T) []T {
	copyValue := make([]T, len(value))
	copy(copyValue, value)
	return copyValue
}
func CopySlicess[T any](value [][]T) [][]T {
	copyValue := make([][]T, len(value))
	for i := 0; i < len(value); i++ {
		copyValue[i] = CopySlices(value[i])
	}
	return copyValue
}

func GetContentTypeWithBytes(content []byte) string {
	return http.DetectContentType(content)
}

func Uuid() uuid.UUID {
	return uuid.New()
}
func ImgDiffer(c, c2 []byte) (float64, error) {
	img1, _, err := image.Decode(bytes.NewBuffer(c))
	if err != nil {
		return 0, err
	}
	img2, _, err := image.Decode(bytes.NewBuffer(c2))
	if err != nil {
		return 0, err
	}
	var score float64
	bounds := img1.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()
			score += math.Pow(float64(r1)-float64(r2), 2)
			score += math.Pow(float64(g1)-float64(g2), 2)
			score += math.Pow(float64(b1)-float64(b2), 2)
		}
	}
	score /= math.Pow(2, 16) * math.Pow(float64(bounds.Dx()), 2) * math.Pow(float64(bounds.Dy()), 2)
	return score, nil
}
func Signal(preCtx context.Context, fun func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch,
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP,
		syscall.SIGABRT, syscall.SIGBUS, syscall.SIGFPE, syscall.SIGSEGV, syscall.SIGPIPE,
		syscall.SIGALRM, syscall.SIGTERM)
	select {
	case <-preCtx.Done():
		if fun != nil {
			fun()
		}
		signal.Stop(ch)
	case s := <-ch:
		if fun != nil {
			fun()
		}
		signal.Stop(ch)
		signal.Reset(s)
		if p, err := os.FindProcess(os.Getpid()); err == nil && p != nil {
			sg, ok := s.(syscall.Signal)
			if ok && sg == syscall.SIGINT {
				p.Signal(syscall.SIGKILL)
			} else {
				p.Signal(s)
			}
		}
	}
}
