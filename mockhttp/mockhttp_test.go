/*
 Copyright 2020 Qiniu Limited (qiniu.com)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package mockhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/qiniu/x/mockhttp"
	"github.com/qiniu/x/rpc"
)

// --------------------------------------------------------------------

func reply(w http.ResponseWriter, code int, data interface{}) {
	msg, _ := json.Marshal(data)
	h := w.Header()
	h.Set("Content-Length", strconv.Itoa(len(msg)))
	h.Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(msg)
}

// --------------------------------------------------------------------

type FooRet struct {
	A int    `json:"a"`
	B string `json:"b"`
	C string `json:"c"`
}

type HandleRet map[string]string

type FooServer struct{}

func (p *FooServer) foo(w http.ResponseWriter, req *http.Request) {
	reply(w, 200, &FooRet{1, req.Host, req.URL.Path})
}

func (p *FooServer) handle(w http.ResponseWriter, req *http.Request) {
	reply(w, 200, HandleRet{"foo": "1", "bar": "2"})
}

func (p *FooServer) postDump(w http.ResponseWriter, req *http.Request) {
	io.Copy(w, req.Body)
}

func (p *FooServer) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/foo", func(w http.ResponseWriter, req *http.Request) { p.foo(w, req) })
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) { p.handle(w, req) })
	mux.HandleFunc("/dump", func(w http.ResponseWriter, req *http.Request) { p.postDump(w, req) })
}

// --------------------------------------------------------------------

func TestBasic(t *testing.T) {

	server := new(FooServer)
	server.RegisterHandlers(http.DefaultServeMux)

	mockhttp.DefaultTransport.SetRemoteAddr("127.0.0.1:8080")
	mockhttp.ListenAndServe("foo.com", nil)

	ctx := context.TODO()
	c := rpc.Client{Client: mockhttp.DefaultClient}
	{
		var foo FooRet
		err := c.Call(ctx, &foo, "POST", "http://foo.com/foo")
		if err != nil {
			t.Fatal("call foo failed:", err)
		}
		if foo.A != 1 || foo.B != "foo.com" || foo.C != "/foo" {
			t.Fatal("call foo: invalid ret")
		}
		fmt.Println(foo)
	}
	{
		var ret map[string]string
		err := c.Call(ctx, &ret, "POST", "http://foo.com/bar")
		if err != nil {
			t.Fatal("call foo failed:", err)
		}
		if ret["foo"] != "1" || ret["bar"] != "2" {
			t.Fatal("call bar: invalid ret")
		}
		fmt.Println(ret)
	}
	{
		resp, err := c.Post("http://foo.com/dump", "", nil)
		if err != nil {
			t.Fatal("post foo failed:", err)
		}
		resp.Body.Close()
		resp, err = c.Post("http://foo.com/dump", "", strings.NewReader("hello"))
		if err != nil {
			t.Fatal("post foo failed:", err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal("ioutil.ReadAll:", err)
		}
		if v := string(b); v != "hello" {
			t.Fatal("body:", v)
		}
	}
}

func TestErrRoundTrip(t *testing.T) {
	tr := mockhttp.NewTransport()
	if _, err := tr.RoundTrip(&http.Request{
		URL: &url.URL{Host: "unknown.com"},
	}); err != mockhttp.ErrServerNotFound {
		t.Fatal("TestErrRoundTrip:", err)
	}
}

// --------------------------------------------------------------------
