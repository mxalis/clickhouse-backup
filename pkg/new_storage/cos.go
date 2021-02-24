package new_storage

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/AlexAkulov/clickhouse-backup/config"

	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/tencentyun/cos-go-sdk-v5/debug"
)

type COS struct {
	client *cos.Client
	Config *config.COSConfig
	Debug  bool
}

// Connect - connect to cos
func (c *COS) Connect() error {
	u, err := url.Parse(c.Config.RowURL)
	if err != nil {
		return err
	}
	b := &cos.BaseURL{BucketURL: u}
	timeout, err := time.ParseDuration(c.Config.Timeout)
	if err != nil {
		return err
	}
	c.client = cos.NewClient(b, &http.Client{
		Timeout: timeout,
		Transport: &cos.AuthorizationTransport{
			SecretID:  c.Config.SecretID,
			SecretKey: c.Config.SecretKey,
			// request debug
			Transport: &debug.DebugRequestTransport{
				RequestHeader:  c.Debug,
				RequestBody:    false,
				ResponseHeader: c.Debug,
				ResponseBody:   false,
			},
		},
	})
	// check bucket exists
	_, err = c.client.Bucket.Head(context.Background())
	return err
}

func (c *COS) Kind() string {
	return "COS"
}

func (c *COS) StatFile(key string) (RemoteFile, error) {
	// file max size is 5Gb
	resp, err := c.client.Object.Get(context.Background(), path.Join(c.Config.Path, key), nil)
	if err != nil {
		cosErr, ok := err.(*cos.ErrorResponse)
		if ok && cosErr.Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	modifiedTime, _ := parseTime(resp.Response.Header.Get("Date"))
	return &cosFile{
		size:         resp.Response.ContentLength,
		name:         resp.Request.URL.Path,
		lastModified: modifiedTime,
	}, nil
}

func (c *COS) DeleteFile(key string) error {
	_, err := c.client.Object.Delete(context.Background(), path.Join(c.Config.Path, key))
	return err
}

func (c *COS) Walk(cosPath string, recursuve bool, process func(RemoteFile) error) error {
	prefix := path.Join(c.Config.Path, cosPath)
	delimiter := ""
	if !recursuve {
		delimiter = "/"
	}
	res, _, err := c.client.Bucket.Get(context.Background(), &cos.BucketGetOptions{
		Delimiter: delimiter,
		Prefix: prefix,
	})
	if err != nil {
		return err
	}
	for _, dir := range res.CommonPrefixes {
		process(&cosFile{
			name: strings.TrimPrefix(dir, prefix),
		})
	}
	for _, v := range res.Contents {
		modifiedTime, _ := parseTime(v.LastModified)
		process(&cosFile{
			name:         strings.TrimPrefix(v.Key, prefix),
			lastModified: modifiedTime,
			size:         int64(v.Size),
		})
	}
	return nil
}

func (c *COS) GetFileReader(key string) (io.ReadCloser, error) {
	resp, err := c.client.Object.Get(context.Background(), path.Join(c.Config.Path, key), nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *COS) PutFile(key string, r io.ReadCloser) error {
	_, err := c.client.Object.Put(context.Background(), path.Join(c.Config.Path, key), r, nil)
	return err
}

type cosFile struct {
	size         int64
	lastModified time.Time
	name         string
}

func (f *cosFile) Size() int64 {
	return f.size
}

func (f *cosFile) Name() string {
	return f.name
}

func (f *cosFile) LastModified() time.Time {
	return f.lastModified
}

func parseTime(text string) (t time.Time, err error) {
	timeFormats := []string{
		"Mon, 02 Jan 2006 15:04:05 GMT",
		time.RFC850,
		time.ANSIC,
		time.RFC3339,
	}

	for _, layout := range timeFormats {
		t, err = time.Parse(layout, text)
		if err == nil {
			return
		}
	}
	return
}
