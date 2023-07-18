package extract

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"gitee.com/baixudong/gospider/bs4"
	"gitee.com/baixudong/gospider/kinds"
	"gitee.com/baixudong/gospider/re"
	"gitee.com/baixudong/gospider/tools"

	"github.com/tidwall/gjson"
)

func jsonLls(data gjson.Result) ([]gjson.Result, bool) {
	lls := []gjson.Result{}
	lls2 := []gjson.Result{}
	for _, val := range data.Map() {
		if val.IsArray() {
			zzs := []gjson.Result{}
			ok := true
			start_txt := ""
			for _, va := range val.Array() {
				if va.IsObject() {
					vks := kinds.NewSet[string]()
					for vk := range va.Map() {
						vks.Add(vk)
					}
					tmpData := vks.Array()
					sort.Strings(tmpData)
					now_txt := strings.Join(tmpData, "##")
					if start_txt == "" {
						start_txt = now_txt
					} else if start_txt != now_txt {
						ok = false
					}
					zzs = append(zzs, va)
				}
			}
			if len(zzs) > 1 && ok {
				if len(zzs) > len(lls2) {
					lls2 = zzs
				}
			} else {
				lls = append(lls, zzs...)
			}
		}
	}
	if len(lls2) > 0 {
		return lls2, true
	}
	return lls, false
}

// 解析json 中的列表
func Json(data gjson.Result) []gjson.Result {
	var lls []gjson.Result
	var ok bool
	if data.IsObject() {
		lls, ok = jsonLls(data)
		if ok {
			return lls
		}
	} else if data.IsArray() {
		lls = data.Array()
	} else {
		return lls
	}
	for len(lls) > 0 {
		zong := []gjson.Result{}
		for _, ll_data := range lls {
			l_datas, ok := jsonLls(ll_data)
			if ok {
				return l_datas
			} else {
				zong = append(zong, l_datas...)
			}
		}
		lls = zong
	}
	return lls
}

type ListOption struct {
	HasTime  bool     //是否有时间
	Keywords []string //列表关键词
}

// 解析dom 列表
func Lis(html *bs4.Client, options ...ListOption) [][]*NodeA {
	option := ListOption{}
	if len(options) > 0 {
		option = options[0]
	}
	aas := getA(html.Find("body").Childrens(), []string{"body"})
	for _, aa := range aas {
		aa.hasTime = option.HasTime
		aa.keywords = option.Keywords
	}
	return clearNode(aas)
}

func sameScore(nodes []*NodeA, nodes2 []*NodeA) float64 {
	var sameNum float64
	if len(nodes) > len(nodes2) {
		nodes, nodes2 = nodes2, nodes
	}
	for i := 0; i < len(nodes); i++ {
		for j := 0; j < len(nodes2); j++ {
			if (nodes[i].Href != "" && nodes[i].Href == nodes2[j].Href) ||
				(nodes[i].Title != "" && nodes[i].Title == nodes2[j].Title) {
				sameNum += 1
				break
			}

		}
	}
	return sameNum / float64(len(nodes))
}

// 解析dom 多个 列表
func Lis2(html *bs4.Client, html2 *bs4.Client, options ...ListOption) [][]*NodeA {
	result := Lis(html, options...)
	result2 := Lis(html2, options...)
	delIndex := []int{}
	for i := 0; i < len(result); i++ {
		for j := 0; j < len(result2); j++ {
			if sameScore(result[i], result2[j]) > 0.7 {
				delIndex = append(delIndex, i)
			}
		}
	}
	datas := tools.DelSliceIndex(result, delIndex...)
	if len(datas) > 0 {
		return datas
	}
	return result
}

// 解析dom 内容
func Content(html *bs4.Client) *bs4.Client {
	var max_score float64
	result := html
	for _, node := range html.ChildrensAll() {
		ti_text := re.Sub(`\s`, "", node.Text())
		ti := len(ti_text)
		for _, img := range node.Finds("img") {
			width := img.Get("width")
			height := img.Get("height")
			widthRs := re.Search(`(\d+)%`, width)
			heightRs := re.Search(`(\d+)%`, height)
			style := re.Sub(`\s`, "", img.Get("style"))
			var w, h float64
			if widthRs != nil {
				w, _ = strconv.ParseFloat(widthRs.Group(1), 64)
				w *= 1000
			} else {
				widthRs = re.Search(`\d+`, width)
				if widthRs != nil {
					w, _ = strconv.ParseFloat(widthRs.Group(), 64)
				} else {
					widthRs = re.Search(`width:(\d+)%`, style)
					if widthRs != nil {
						w, _ = strconv.ParseFloat(widthRs.Group(1), 64)
						w *= 1000
					} else {
						widthRs = re.Search(`width:(\d+)`, style)
						if widthRs != nil {
							w, _ = strconv.ParseFloat(widthRs.Group(1), 64)
						}
					}
				}
			}
			if heightRs != nil {
				h, _ = strconv.ParseFloat(heightRs.Group(1), 64)
				h *= 1000
			} else {
				heightRs = re.Search(`\d+`, height)
				if heightRs != nil {
					h, _ = strconv.ParseFloat(heightRs.Group(), 64)
				} else {
					heightRs = re.Search(`height:(\d+)%`, style)
					if heightRs != nil {
						h, _ = strconv.ParseFloat(heightRs.Group(1), 64)
						h *= 1000
					} else {
						heightRs = re.Search(`height:(\d+)`, style)
						if heightRs != nil {
							h, _ = strconv.ParseFloat(heightRs.Group(1), 64)
						}
					}
				}
			}
			if w > 400 && h > 600 {
				ti += int(w * h / 1000)
			}
		}
		tgi := len(node.ChildrensAll())
		aas := []string{}
		for _, li := range node.Finds("a") {
			aaLiRs := re.Sub(`\s`, "", li.Text())
			aas = append(aas, aaLiRs)
		}
		lti := len(strings.Join(aas, ""))
		ltgi := len(aas)
		var density int
		if (tgi - ltgi) != 0 {
			density = (ti - lti) / (tgi - ltgi)
		}
		text_tag_count := len(node.Finds("div,span,p"))
		sbiRs := re.FindAll(fmt.Sprintf("[%s]", re.Quote(`！，。？、；：“”‘’《》%（）,.?:;'"!%()`)), ti_text)
		sbdi := (ti - lti) / (len(sbiRs) + 1)
		if sbdi == 0 {
			sbdi = 1
		}
		score := float64(density) * math.Log10(float64(text_tag_count)+2) * math.Log(float64(sbdi))
		if score > max_score {
			max_score = score
			result = node
		}
	}
	return result
}
func (obj *NodeA) Score() float64 {
	keywords := []string{"公(?:告|示)|(?:中|招)标|采购|结果"}
	if len(obj.keywords) > 0 {
		keywords = obj.keywords
	}
	var score float64
	var txtLen int
	var haveTitle bool
	var haveKeyword bool
	var haveTime bool

	txt := re.Sub(`\s`, "", obj.Node.Text())
	txtLen += len(txt)
	if obj.Node.Get("title") != "" {
		obj.Title = obj.Node.Get("title")
		haveTitle = true
	} else {
		obj.Title = strings.TrimSpace(obj.Node.Text())
	}
	if obj.Node.Get("href") != "" {
		obj.Href = obj.Node.Get("href")
	}
	for _, keyword := range keywords {
		keywordRs := re.Search(keyword, txt)
		if keywordRs != nil {
			haveKeyword = true
			break
		}
	}
	parent_txt := obj.MainNode().String()
	parent_txt += obj.MainNode().Text()
	gTime := tools.GetTime(parent_txt)
	if gTime != "" {
		obj.Time = gTime
		haveTime = true
	}

	score += float64(txtLen)
	if haveTitle {
		score += 50
	}
	if haveKeyword {
		score += 70
	}
	if haveTime {
		score += 100
	}
	obj.score = score
	return score
}

func panKeys(paths []string, paths2 []string) (bool, int) {
	if len(paths) != len(paths2) {
		return false, 0
	}
	var index int
	var num int
	for path_index, path := range paths {
		if path != paths2[path_index] {
			if strings.Split(path, ",")[0] != strings.Split(paths2[path_index], ",")[0] {
				return false, 0
			}
			num++
			index = path_index
		}
	}
	if num == 1 {
		return true, index
	}
	return false, 0
}

type nodeMap struct {
	index int
	nodes []nodeScore
}

func (obj nodeMap) score() float64 {
	var i_num float64
	for _, i := range obj.nodes {
		i_num += i.score
	}
	i_num = (i_num / float64(len(obj.nodes))) + float64(len(obj.nodes))
	return i_num
}

type nodeSet struct {
	index int
	nodes *kinds.Set[*NodeA]
}
type nodeScore struct {
	score float64
	node  *NodeA
}

func clearNode(results []*NodeA) [][]*NodeA {
	zongLen := len(results)
	groupSets := []nodeSet{}
	for i := 0; i < zongLen-1; i++ {
		for j := i + 1; j < zongLen; j++ {
			same, index := panKeys(results[i].paths, results[j].paths)
			if same {
				haveSame := false
				for k := 0; k < len(groupSets); k++ { //挨个寻找相同的路径
					if groupSets[k].index == index {
						same2, index2 := panKeys(groupSets[k].nodes.Array()[0].paths, results[i].paths)
						if same2 && index == index2 {
							haveSame = true
							groupSets[k].nodes.Add(results[i])
							groupSets[k].nodes.Add(results[j])
						}
					}
				}
				if !haveSame {
					groupSets = append(groupSets, nodeSet{
						index: index,
						nodes: kinds.NewSet(results[i], results[j]),
					})
				}
			}
		}
	}
	groupMaps := []nodeMap{}
	for _, val := range groupSets {
		nodeScors := []nodeScore{}
		for node := range val.nodes.Map() {
			var score float64
			if node.index == val.index && (node.score > 0 || node.index > 0) {
				score = node.score
			} else {
				node.index = val.index
				score = node.Score()
			}
			nodeScors = append(nodeScors, nodeScore{
				score: score,
				node:  node,
			})
		}
		groupMaps = append(groupMaps, nodeMap{
			index: val.index,
			nodes: nodeScors,
		})
	}
	sort.Slice(groupMaps, func(i, j int) bool {
		return groupMaps[i].score() > groupMaps[j].score()
	})
	result := [][]*NodeA{}
	for i := 0; i < len(groupMaps)-1; i++ {
		for j := i + 1; j < len(groupMaps); j++ {
			delIndex := []int{}
			for ii := 0; ii < len(groupMaps[i].nodes); ii++ {
				for jj := 0; jj < len(groupMaps[j].nodes); jj++ {
					if groupMaps[i].nodes[ii].node == groupMaps[j].nodes[jj].node {
						delIndex = append(delIndex, jj)
					}
				}
			}
			if len(delIndex) > 0 {
				groupMaps[j].nodes = tools.DelSliceIndex(groupMaps[j].nodes, delIndex...)
			}
		}
		if len(groupMaps[i].nodes) > 1 {
			sort.Slice(groupMaps[i].nodes, func(i, j int) bool {
				path := groupMaps[i].nodes[i].node.paths
				path2 := groupMaps[i].nodes[j].node.paths

				path_len := len(path)
				path2_len := len(path2)
				if path_len != path2_len {
					return path_len < path2_len
				}
				var max_len int
				var min_len int
				if path_len > path2_len {
					max_len = path_len
					min_len = path2_len
				} else {
					max_len = path2_len
					min_len = path_len
				}
				for num := 0; num < max_len; num++ {
					if num >= min_len {
						return path_len < path2_len
					}
					path_lls := strings.Split(path[num], ",")
					path2_lls := strings.Split(path2[num], ",")

					if path_lls[0] != path2_lls[0] {
						return path_lls[0] < path2_lls[0]
					}
					if len(path_lls) > 1 && len(path2_lls) > 1 && path_lls[1] != path2_lls[1] {
						path_num, _ := strconv.Atoi(path_lls[1])
						path2_num, _ := strconv.Atoi(path2_lls[1])
						return path_num < path2_num
					}
				}
				return false
			})
			tempSlice := []*NodeA{}
			for _, node := range groupMaps[i].nodes {
				tempSlice = append(tempSlice, node.node)
			}
			result = append(result, tempSlice)
		}
	}
	return result
}

type NodeA struct {
	Node     *bs4.Client //当前节点
	Time     string      //发布时间
	Href     string      //发布地址
	Title    string      //标题
	paths    []string    //路径
	score    float64     //节点分数
	index    int         //路径主节点下标
	hasTime  bool        //是否有时间
	keywords []string    //列表关键词
}

func (obj *NodeA) MainNode() *bs4.Client { //返回主节点
	rangeNum := len(obj.paths) - obj.index - 1
	if rangeNum == 0 {
		return obj.Node
	} else {
		var mainNode *bs4.Client
		for n := 0; n < rangeNum; n++ {
			if mainNode == nil {
				mainNode = obj.Node.Parent()
			} else {
				mainNode = mainNode.Parent()
			}
		}
		return mainNode
	}
}
func getA(htmls []*bs4.Client, paths []string) []*NodeA {
	results := []*NodeA{}
	for html_index, html := range htmls {
		html_name := html.Name()
		now_paths := append(append([]string{}, paths...), fmt.Sprintf("%s,%d", html_name, html_index))
		if html_name == "a" {
			results = append(results, &NodeA{
				Node:  html,
				paths: now_paths,
			})
		} else {
			results = append(results, getA(html.Childrens(), now_paths)...)
		}
	}
	return results
}
