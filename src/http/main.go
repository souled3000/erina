package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	//	url := "http://123.57.47.53:10004/auth/generatekey?userId=224&token=SPg4dg1J"
	url := "http://123.57.47.53:10004/auth/generatekey"
	r := make(map[string]string)
	r["userId"] = "237"
	r["token"] = "qIM8woQo"
	_, re, _ := get(url, r)
	fmt.Println(re)
	if v, ok := re["data"]; ok {
		if data, ok := v.(map[string]interface{}); ok {
			if status, ok := data["status"].(float64); ok {
				fmt.Println(status)
			}
			if key, ok := data["key"].(string); ok {
				fmt.Println(key)
			}
		}
	}
}
func get(url string, input map[string]string) (status int32, response map[string]interface{}, err error) {
	reqs := ""
	for k, v := range input {
		reqs = fmt.Sprintf("%s&%s=%s", reqs, k, v)
	}
	rep, err := http.Get(url + "?" + reqs)
	if err != nil {
		return -1, nil, fmt.Errorf("[POST] input:[%#v] failed on server,response:%v,error: %v", input, rep, err)
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
