# 快速开始

```go
package main

import (
	"log"

	"gitee.com/baixudong/gospider/bs4"
)
func main() {
	html := bs4.NewClient("<div>hellow</div>")
	div := html.Find("div")
	log.Print(div.Text()) //hellow
}
```

