//go:build !consulent
// +build !consulent

package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/testrpc"
)

func TestHTTP_Peering_GenerateToken_OSS_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("Doesn't allow partitions in OSS HTTP requests", func(t *testing.T) {
		reqBody := &pbpeering.GenerateTokenRequest{
			PeerName: "peering-a",
		}
		reqBodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)
		req, err := http.NewRequest("POST", "/v1/peering/token?partition=foo",
			bytes.NewReader(reqBodyBytes))
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		a.srv.h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		body, _ := io.ReadAll(resp.Body)
		require.Contains(t, string(body), "Partitions are a Consul Enterprise feature")
	})
}
