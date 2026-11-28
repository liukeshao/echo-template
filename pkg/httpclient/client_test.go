package httpclient_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/pkg/httpclient"
)

func TestNew_DefaultsAndCustomConfig(t *testing.T) {
	t.Parallel()

	c := httpclient.New(httpclient.Config{})
	assert.NotNil(t, c.Client)
	assert.Equal(t, 10*time.Second, c.Timeout)
	assert.NotSame(t, http.DefaultTransport, c.Transport)

	tr, ok := c.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 100, tr.MaxIdleConns)
	assert.Equal(t, 20, tr.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, tr.IdleConnTimeout)

	custom := httpclient.New(httpclient.Config{
		Timeout:             3 * time.Second,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	})
	assert.Equal(t, 3*time.Second, custom.Timeout)
	customTr, ok := custom.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 50, customTr.MaxIdleConns)
	assert.Equal(t, 10, customTr.MaxIdleConnsPerHost)
	assert.Equal(t, 30*time.Second, customTr.IdleConnTimeout)
}
