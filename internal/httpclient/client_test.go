package httpclient_test

import (
	"net/http"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/echo-template/internal/httpclient"
)

func TestNew_ClonedTunedTransport(t *testing.T) {
	t.Parallel()
	inj := do.New(httpclient.Package)
	c := do.MustInvoke[*httpclient.Client](inj)

	assert.NotNil(t, c.Client)
	assert.NotSame(t, http.DefaultTransport, c.Transport)
	tr, ok := c.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 100, tr.MaxIdleConns)
	assert.Equal(t, 20, tr.MaxIdleConnsPerHost)
}
