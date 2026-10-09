package common

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticToken struct{}

func (staticToken) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// hangFirstTransport blocks the first request until its context is cancelled and answers later ones.
type hangFirstTransport struct {
	calls atomic.Int32
}

func (t *hangFirstTransport) Do(req *http.Request) (*http.Response, error) {
	if t.calls.Add(1) == 1 {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"name":"q","properties":{"status":"Disabled"}}`)),
		Request:    req,
	}, nil
}

func TestArmClientOptions_SetsTryTimeout(t *testing.T) {
	assert.Equal(t, armTryTimeout, ArmClientOptions().Retry.TryTimeout)
}

func TestArmClientOptions_RetriesHungAttempt(t *testing.T) {
	transport := &hangFirstTransport{}
	options := ArmClientOptions()
	options.Retry.TryTimeout = 100 * time.Millisecond
	options.Retry.RetryDelay = time.Millisecond
	options.Transport = transport

	client, err := armservicebus.NewQueuesClient("sub", staticToken{}, options)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.CreateOrUpdate(ctx, "rg", "ns", "q", armservicebus.SBQueue{}, nil)

	require.NoError(t, err)
	assert.Equal(t, int32(2), transport.calls.Load())
}
