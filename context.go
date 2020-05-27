package yee

import (
	"encoding/json"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Context interface {
	Request() *http.Request
	Response() http.ResponseWriter
	HTML(code int, html string) (err error)
	JSON(code int, i interface{}) error
	String(code int, s string) error
	Status(code int)
	QueryParam(name string) string
	QueryString() string
	SetHeader(key string, value string)
	AddHeader(key string, value string)
	GetHeader(key string) string
	FormValue(name string) string
	FormParams() (url.Values, error)
	FormFile(name string) (*multipart.FileHeader, error)
	MultipartForm() (*multipart.Form, error)
	Redirect(code int, uri string) error
	Params(name string) string
	RequestURI() string
	Scheme() string
	IsTls() bool
	Next()
	HTMLTml(code int, tml string) (err error)
	QueryParams() map[string][]string
	Bind(i interface{}) error
	GetMethod() string
	Get(key string) interface{}
	Put(key string, values interface{})
	MiddError(code int,err error) error
}

type context struct {
	w         http.ResponseWriter
	r         *http.Request
	path      string
	method    string
	code      int
	queryList url.Values // 保存QueryParam
	params    map[string]string

	// middleware
	handlers []HandlerFunc
	index    int
	store    map[string]interface{}
	lock     sync.RWMutex
	noRewrite bool

	intercept bool
}

func newContext(w http.ResponseWriter, r *http.Request) *context {
	return &context{
		w:      w,
		r:      r,
		path:   r.URL.Path,
		method: r.Method,
		index:  -1,
	}
}

func (c *context) Next()  {
	c.index++
	s := len(c.handlers)
	for ; c.index < s; c.index++ {
		if c.intercept {
			break
		}
		if err := c.handlers[c.index].Func(c);err != nil {
				c.intercept = true
		}
	}
}

func (c *context) MiddError(code int,err error) error {
	_ = c.String(code, err.Error())
	return err
}

func (c *context) Put(key string, values interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.store == nil {
		c.store = make(map[string]interface{})
	}
	c.store[key] = values
}

func (c *context) Get(key string) interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.store[key]
}

func (c *context) GetMethod() string {
	return c.r.Method
}

func (c *context) Request() *http.Request {
	return c.r
}

func (c *context) Response() http.ResponseWriter {
	return c.w
}

func (c *context) HTML(code int, html string) (err error) {
	return c.HTMLBlob(code, []byte(html))
}

func (c *context) HTMLTml(code int, tml string) (err error) {
	s, e := ioutil.ReadFile(tml)
	if e != nil {
		panic(e)
	}
	return c.HTMLBlob(code, s)
}

func (c *context) HTMLBlob(code int, b []byte) (err error) {
	return c.Blob(code, MIMETextHTMLCharsetUTF8, b)
}

func (c *context) Blob(code int, contentType string, b []byte) (err error) {
	c.writeContentType(contentType)
	c.w.WriteHeader(code)
	_, err = c.w.Write(b)
	return
}

func (c *context) JSON(code int, i interface{}) error {
	enc := json.NewEncoder(c.w)
	c.writeContentType(MIMEApplicationJSONCharsetUTF8)
	c.w.WriteHeader(code)
	return enc.Encode(i)
}

func (c *context) String(code int, s string) error {
	return c.Blob(code, MIMETextPlainCharsetUTF8, []byte(s))
}

func (c *context) Status(code int) {
	c.w.WriteHeader(code)
}

func (c *context) SetHeader(key string, value string) {
	c.w.Header().Set(key, value)
}

func (c *context) AddHeader(key string, value string) {
	c.w.Header().Add(key, value)
}

func (c *context) GetHeader(key string) string {
	return c.w.Header().Get(key)
}

func (c *context) Params(name string) string {
	v, _ := c.params[name]
	return v
}

func (c *context) QueryParams() map[string][]string {
	return c.r.URL.Query()
}

func (c *context) QueryParam(name string) string {
	// 判断queryList是否为空,如果为空 调用c.r.URL.Query() 方法获取URL Query值
	if c.queryList == nil {
		c.queryList = c.r.URL.Query()
	}
	return c.queryList.Get(name)
}

func (c *context) QueryString() string {
	return c.r.URL.RawQuery
}

func (c *context) FormValue(name string) string {
	return c.r.FormValue(name)
}

func (c *context) FormParams() (url.Values, error) {
	if strings.HasPrefix(c.r.Header.Get(HeaderContentType), MIMEMultipartForm) {
		if err := c.r.ParseMultipartForm(defaultMemory); err != nil {
			return nil, err
		}
	} else {
		if err := c.r.ParseForm(); err != nil {
			return nil, err
		}
	}
	return c.r.Form, nil
}

func (c *context) FormFile(name string) (*multipart.FileHeader, error) {
	_, fd, err := c.r.FormFile(name)
	return fd, err
}

func (c *context) MultipartForm() (*multipart.Form, error) {
	err := c.r.ParseMultipartForm(defaultMemory)
	return c.r.MultipartForm, err
}

func (c *context) RequestURI() string {
	return c.r.RequestURI
}

func (c *context) Scheme() string {
	scheme := "http"
	if scheme := c.r.Header.Get(HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := c.r.Header.Get(HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := c.r.Header.Get(HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := c.r.Header.Get(HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return scheme
}

func (c *context) IsTls() bool {
	return c.r.TLS != nil
}

func (c *context) Redirect(code int, uri string) error {
	if code < 300 || code > 308 {
		return ErrInvalidRedirectCode
	}
	c.r.Header.Set("Location", uri)
	c.w.WriteHeader(code)
	return nil
}

func (c *context) writeContentType(value string) {
	header := c.w.Header()
	if header.Get(HeaderContentType) == "" {
		header.Set(HeaderContentType, value)
	}
}