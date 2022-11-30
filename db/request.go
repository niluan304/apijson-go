package db

import (
	"github.com/glennliao/apijson-go/config"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"strings"
)

var requestMap = map[string]Request{}

type Request struct {
	Debug     int8
	Version   int16
	Method    string
	Tag       string
	Structure g.Map
	Detail    string
	CreatedAt *gtime.Time

	ExecQueue []string
}

func loadRequestMap() {
	_requestMap := make(map[string]Request)

	var requestList []Request
	err := g.DB().Model(config.TableRequest).Scan(&requestList)
	if err != nil {
		panic(err)
	}

	for _, item := range requestList {

		tag := item.Tag
		if strings.HasSuffix(tag, "[]") {
			tag = tag[0 : len(tag)-2]
		}
		if strings.ToLower(tag) != tag {
			// 本身大写, 如果没有外层, 则套一层
			if _, ok := item.Structure[tag]; !ok {
				item.Structure = g.Map{
					tag: item.Structure,
				}
			}
		}

		// todo 改成列表读取数据库, 避免多次查询
		type ext struct {
			ExecQueue string
		}
		var _ext *ext
		_ = g.DB().Model(config.TableRequestExt).Where(g.Map{
			"version": item.Version,
			"method":  item.Method,
			"tag":     item.Tag,
		}).Scan(&_ext)

		if _ext != nil {
			item.ExecQueue = strings.Split(_ext.ExecQueue, ",")
		} else {
			tag := item.Tag
			if strings.HasSuffix(tag, "[]") {
				tag = tag[0 : len(tag)-2]
			}
			item.ExecQueue = strings.Split(tag, ",")
		}

		_requestMap[item.Method+"@"+item.Tag+"@"+gconv.String(item.Version)] = item
		// todo 暂按照列表获取, 最后一个是最新, 这里需要调整
		_requestMap[item.Method+"@"+item.Tag+"@"+"latest"] = item
	}

	requestMap = _requestMap
}

func GetRequest(tag string, method string, version string) (*Request, error) {

	if version == "" || version == "-1" || version == "0" {
		version = "latest"
	}

	key := method + "@" + tag + "@" + version
	request, ok := requestMap[key]

	if !ok {
		return nil, gerror.Newf("request[%s]: 404", key)
	}

	return &request, nil
}
