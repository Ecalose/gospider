package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"

	"gitee.com/baixudong/gospider/requests"
	"gitee.com/baixudong/gospider/tools"
	"github.com/tidwall/gjson"
)

type Client struct {
	reqCli  *requests.Client
	baseUrl string
}

type ClientOption struct {
	Host string
	Port int
	Usr  string
	Pwd  string
	Ssl  bool
}

func getBaseUrl(option ClientOption) (string, error) {
	var baseUrl string
	if option.Ssl {
		baseUrl += "https://"
	} else {
		baseUrl += "http://"
	}
	if option.Usr != "" && option.Pwd != "" {
		baseUrl += fmt.Sprintf("%s:%s@%s", option.Usr, option.Pwd, net.JoinHostPort(option.Host, strconv.Itoa(option.Port)))
	} else {
		baseUrl += net.JoinHostPort(option.Host, strconv.Itoa(option.Port))
	}
	return baseUrl, nil
}

func (obj Client) Ping(ctx context.Context) error {
	_, err := obj.reqCli.Request(ctx, "get", obj.baseUrl)
	if err != nil {
		return err
	}
	return nil
}

type UpdateData struct {
	Index string
	Id    string
	Data  any
}
type DeleteData struct {
	Index string
	Id    string
}
type SearchResult struct {
	Total int64
	Datas []gjson.Result
}

func (obj *Client) Count(ctx context.Context, index string, data any) (int64, error) {
	url := obj.baseUrl + fmt.Sprintf("/%s/_count", index)
	jsonData, err := tools.Any2json(data)
	if err != nil {
		return 0, err
	}
	rs, err := obj.reqCli.Request(ctx, "post", url, requests.RequestOption{Json: jsonData.Value()})
	if err != nil {
		return 0, err
	}
	if jsonData, err = rs.Json(); err != nil {
		return 0, err
	}
	if jsonData.Get("error").Exists() {
		return 0, fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	countRs := jsonData.Get("count")
	if !countRs.Exists() {
		return 0, errors.New("not found count")
	}
	return countRs.Int(), nil
}
func (obj *Client) Search(ctx context.Context, index string, data any) (SearchResult, error) {
	var searchResult SearchResult
	url := obj.baseUrl + fmt.Sprintf("/%s/_search", index)
	jsonData, err := tools.Any2json(data)
	if err != nil {
		return searchResult, err
	}
	rs, err := obj.reqCli.Request(ctx, "post", url, requests.RequestOption{Json: jsonData.Value()})
	if err != nil {
		return searchResult, err
	}
	jsonData, err = rs.Json()
	if err != nil {
		return searchResult, err
	}
	if jsonData.Get("error").Exists() {
		return searchResult, fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	hits := jsonData.Get("hits")
	if !hits.Exists() {
		return searchResult, errors.New("not found hits")
	}
	searchResult.Total = hits.Get("total.value").Int()
	searchResult.Datas = hits.Get("hits").Array()
	return searchResult, nil
}
func (obj *Client) Exists(ctx context.Context, index, id string) (bool, error) {
	url := obj.baseUrl + fmt.Sprintf("/%s/_count?q=_id:%s", index, id)
	rs, err := obj.reqCli.Request(ctx, "get", url)
	if err != nil {
		return false, err
	}
	jsonData, err := rs.Json()
	if err != nil {
		return false, err
	}
	if jsonData.Get("error").Exists() {
		return false, fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	countRs := jsonData.Get("count")
	if !countRs.Exists() {
		return false, errors.New("not found count")
	}
	if countRs.Int() > 0 {
		return true, nil
	}
	return false, nil
}
func (obj *Client) Delete(ctx context.Context, deleteData DeleteData, deleteDatas ...DeleteData) error {
	if len(deleteDatas) == 0 {
		return obj.delete(ctx, deleteData)
	}
	return obj.deletes(ctx, append(deleteDatas, deleteData))
}
func (obj *Client) Update(ctx context.Context, updateData UpdateData, updateDatas ...UpdateData) error {
	if len(updateDatas) == 0 {
		return obj.update(ctx, updateData, false)
	}
	return obj.updates(ctx, append(updateDatas, updateData), false)
}
func (obj *Client) Upsert(ctx context.Context, updateData UpdateData, updateDatas ...UpdateData) error {
	if len(updateDatas) == 0 {
		return obj.update(ctx, updateData, true)
	}
	return obj.updates(ctx, append(updateDatas, updateData), true)
}
func (obj *Client) delete(ctx context.Context, deleteData DeleteData) error {
	url := obj.baseUrl + fmt.Sprintf("/%s/_doc/%s", deleteData.Index, deleteData.Id)
	rs, err := obj.reqCli.Request(ctx, "delete", url)
	if err != nil {
		return err
	}
	jsonData, err := rs.Json()
	if err != nil {
		return err
	}
	if jsonData.Get("error").Exists() {
		return fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	return nil
}
func (obj *Client) deletes(ctx context.Context, deleteDatas []DeleteData) error {
	var body bytes.Buffer
	for _, deleteData := range deleteDatas {
		_, err := body.WriteString(fmt.Sprintf(`{"delete":{"_index":"%s","_id":"%s"}}`, deleteData.Index, deleteData.Id))
		if err != nil {
			return err
		}
		_, err = body.WriteString("\n")
		if err != nil {
			return err
		}
	}
	url := obj.baseUrl + "/_bulk"
	rs, err := obj.reqCli.Request(ctx, "post", url, requests.RequestOption{Json: body.Bytes()})
	if err != nil {
		return err
	}
	jsonData, err := rs.Json()
	if err != nil {
		return err
	}
	if jsonData.Get("error").Exists() {
		return fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	return nil
}
func (obj *Client) update(ctx context.Context, updateData UpdateData, upsert bool) error {
	jsonData, err := tools.Any2json(updateData.Data)
	if err != nil {
		return err
	}
	body := map[string]any{
		"doc": jsonData.Value(),
	}
	if upsert {
		body["doc_as_upsert"] = true
	}
	url := obj.baseUrl + fmt.Sprintf("/%s/_doc/%s/_update", updateData.Index, updateData.Id)
	rs, err := obj.reqCli.Request(ctx, "post", url, requests.RequestOption{Json: body})
	if err != nil {
		return err
	}
	if jsonData, err = rs.Json(); err != nil {
		return err
	}
	if jsonData.Get("error").Exists() {
		return fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	return nil
}
func (obj *Client) updates(ctx context.Context, updateDatas []UpdateData, upsert bool) error {
	var body bytes.Buffer
	for _, updateData := range updateDatas {
		_, err := body.WriteString(fmt.Sprintf(`{"update":{"_index":"%s","_id":"%s"}}`, updateData.Index, updateData.Id))
		if err != nil {
			return err
		}
		jsonData, err := tools.Any2json(updateData.Data)
		if err != nil {
			return err
		}
		tempBody := map[string]any{
			"doc": jsonData.Value(),
		}
		if upsert {
			tempBody["doc_as_upsert"] = true
		}
		con, err := json.Marshal(tempBody)
		if err != nil {
			return err
		}
		_, err = body.WriteString("\n")
		if err != nil {
			return err
		}
		_, err = body.Write(con)
		if err != nil {
			return err
		}
		_, err = body.WriteString("\n")
		if err != nil {
			return err
		}
	}
	url := obj.baseUrl + "/_bulk"
	rs, err := obj.reqCli.Request(ctx, "post", url, requests.RequestOption{Json: body.Bytes()})
	if err != nil {
		return err
	}
	jsonData, err := rs.Json()
	if err != nil {
		return err
	}
	if jsonData.Get("error").Exists() {
		return fmt.Errorf("%s >>> %s",
			jsonData.Get("error.type").String(),
			jsonData.Get("error.reason").String())
	}
	return nil
}
func NewClient(ctx context.Context, option ClientOption) (*Client, error) {
	var client Client
	var err error
	if client.reqCli, err = requests.NewClient(ctx); err != nil {
		return nil, err
	}
	baseUrl, err := getBaseUrl(option)
	if err != nil {
		return nil, err
	}
	client.baseUrl = baseUrl
	return &client, client.Ping(ctx)
}
