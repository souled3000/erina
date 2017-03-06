package srv

import (
	"bytes"
	"encoding/json"
	"fmt"
	//"io"
	//	"net"
	"net/http"
	//"net/url"
	"strings"
	//"github.com/golang/glog"
)

var (
	DAckOk          int32 = 0
	DAckHTTPError   int32 = 500
	DAckServerError int32 = 503

	DAckBadCmd int32 = 1002
)

func post(url string, input map[string]string) (status int32, response map[string]interface{}, err error) {
	reqs := ""
	for k, v := range input {
		reqs = fmt.Sprintf("%s&%s=%s", reqs, k, v)
	}
	req := bytes.NewReader([]byte(strings.TrimLeft(reqs, "&")))
	rep, err := http.Post(url, "application/x-www-form-urlencoded;charset=utf-8", req)
	if err != nil {
		return DAckServerError, nil, fmt.Errorf("[POST] input:[%#v] failed on server,response:%v,error: %v", input, rep, err)
	}
	defer rep.Body.Close()

	if rep.StatusCode != 200 {
		return int32(rep.StatusCode), nil, fmt.Errorf("[POST] input:[%#v] failed,http code: %v", input, rep.StatusCode)
	}

	d := json.NewDecoder(rep.Body)
	response = make(map[string]interface{})
	err = d.Decode(&response)
	if err != nil {
		return int32(rep.StatusCode), nil, fmt.Errorf("[POST] %v", err)
	}

	return int32(rep.StatusCode), response, nil
}
func get(url string, input map[string]string) (status int32, response map[string]interface{}, err error) {
	reqs := ""
	for k, v := range input {
		reqs = fmt.Sprintf("%s&%s=%s", reqs, k, v)
	}
	rep, err := http.Get(url + "?" + reqs)
	if err != nil {
		return DAckServerError, nil, fmt.Errorf("[POST] input:[%#v] failed on server,response:%v,error: %v", input, rep, err)
	}
	defer rep.Body.Close()

	if rep.StatusCode != 200 {
		return int32(rep.StatusCode), nil, fmt.Errorf("[POST] input:[%#v] failed,http code: %v", input, rep.StatusCode)
	}

	d := json.NewDecoder(rep.Body)
	response = make(map[string]interface{})
	err = d.Decode(&response)
	if err != nil {
		return int32(rep.StatusCode), nil, fmt.Errorf("[POST] %v", err)
	}

	return int32(rep.StatusCode), response, nil
}
