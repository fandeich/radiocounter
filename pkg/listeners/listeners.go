package listeners

import (
	"context"
	"encoding/json"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"net/http"
	"radio_counter/pkg/tracing"
	"strconv"
	"time"
)

const (
	RadioHeart = "https://a4.radioheart.ru/api/json?userlogin=user8042&api=current_listeners"
	MyRadio24  = "http://myradio24.com/users/meganight/status.json"
)

func GetListenersRadioHeart(ctx context.Context, name string) (num int, err error) {
	ctx, span := tracing.MakeSpanGet(ctx, "MyRadio24")
	defer span.Finish()

	ctx = opentracing.ContextWithSpan(ctx, span)

	url := RadioHeart
	var netClient = &http.Client{
		Timeout: time.Second * 20,
	}

	req, _ := http.NewRequest("GET", url, nil)

	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, url)
	ext.HTTPMethod.Set(span, "GET")

	res, err := netClient.Do(req)
	if err != nil {
		ext.Error.Set(span, true)
		return -1, err
	}
	ext.Error.Set(span, false)

	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)
	num, _ = strconv.Atoi(result[name].(string))

	return
}

func GetListenersMyRadio24(ctx context.Context, name string) (num int, err error) {
	ctx, span := tracing.MakeSpanGet(ctx, "MyRadio24")
	defer span.Finish()
	url := MyRadio24
	var netClient = &http.Client{
		Timeout: time.Second * 20,
	}

	req, _ := http.NewRequest("GET", url, nil)

	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, url)
	ext.HTTPMethod.Set(span, "GET")

	res, err := netClient.Do(req)
	if err != nil {
		ext.Error.Set(span, true)
		return -1, err
	}
	ext.Error.Set(span, false)

	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)
	num = int(result[name].(float64))

	return
}

func GetListeners(ctx context.Context) (num int, err error) {
	ctx, span := tracing.MakeSpanGet(ctx, "GetListener")
	defer span.Finish()

	num1, err1 := GetListenersRadioHeart(ctx, "listeners")
	num2, err2 := GetListenersMyRadio24(ctx, "listeners")
	if err1 != nil {
		err = err1
	}
	if err2 != nil {
		err = err2
	}
	num = num1 + num2
	if num < 0 {
		num = 0
	}
	span.SetTag("CountListeners", num)
	span.SetTag("Minute", time.Now().Minute())
	if err != nil {
		ext.Error.Set(span, false)
	} else {
		ext.Error.Set(span, true)
	}
	return
}
