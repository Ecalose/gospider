# 简介
python到golang爬虫过渡的所有必需库.
1. [请求库](../../tree/master/requests)：支持ja3,http2 协议,各种主流代理协议,覆盖python requests 所有的功能
2. [代理库](../../tree/master/proxy)：对数据抓包拦截修改,聚合代理池为隧道代理,作为网关拦截爬虫请求,作为代理突破ja3反爬
3. [并发库](../../tree/master/thread)：自实现高性能并发库
4. [执行js](../../tree/master/cmd): 通过管道调用js 方法
5. [执行python](../../tree/master/cmd): 通过管道调用python 方法
6. 更多功能...
# 依赖
* go1.20 (不要低于这个版本)
# 安装 (不要拉github的包,go包路径只能在gitee和github选一个,拉github包会出现路径问题)
```
go get -u gitee.com/baixudong/gospider
```
# 文档
文档请去各模块文件夹下寻找,如果出现文档有bug,文档不完善 请联系作者,或者提交issues

# 使用本项目已完成的爬虫
* 为全国招投标上万个网站的爬虫提供渲染服务，dns 缓存服务
* 快如闪电的VPN
* 快手滑块,淘宝滑块,抖音滑块cookies获取爬虫
* 微软文字转语音,火山文字转语音爬虫,迅捷语言识别爬虫,网易见外爬虫,阿里云长文本实时转语音SDK爬虫
* b站，知乎，好看，抖音，快手，西瓜，火山的视频解析爬虫
* 抖音视频,评论抓取爬虫
* akamai 德州仪器下单,监控,欧时rs自动下单，贸泽自动下单爬虫
* 企查查爬虫爬虫,天眼查爬虫爬虫,钉钉爬虫
* 114黄页,88黄页,顺企网,慧聪网,爱采购,258.com,百姓网,51网,金泉网,传众网,八方资源网,阿拉伯网站,顺时网,1688 爬虫
* 百度地图爬虫,高德地图爬虫,腾讯地图爬虫,大众点评爬虫,小红书爬虫。
* 掌声高考爬虫，百度高考爬虫，夸克高考爬虫
* 百度翻译爬虫，百度ai 爬虫
* 五秒盾cookies,瑞数爬虫
* new bing 爬虫
* 微擎爬虫,狂团爬虫
* 知乎数据抓取和数据分析,知乎自动化点赞爬虫
* 办公资源网爬虫
* 古诗词数据爬虫
* 国内会展爬虫
* 押韵助手爬虫
* chegg爬虫
* csdn 爬虫
# 商业合作
|||
|-|-|
|微信.手机|17626043715|
|qq|2216403312|

# 教程
1. [知乎](https://www.zhihu.com/people/xiao-bai-shu-87-3/posts)
2. [掘金](https://juejin.cn/user/4098624347452359/posts)
3. [csdn](https://blog.csdn.net/Mr_bai_404?type=blog)

![](im.jpg)