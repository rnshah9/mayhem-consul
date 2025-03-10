package peering

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// same certificate that appears in our connect tests
var validCA = `
-----BEGIN CERTIFICATE-----
MIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg
Q0EgNzAeFw0xODA1MjExNjMzMjhaFw0yODA1MTgxNjMzMjhaMBYxFDASBgNVBAMT
C0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAER0qlxjnRcMEr
iSGlH7G7dYU7lzBEmLUSMZkyBbClmyV8+e8WANemjn+PLnCr40If9cmpr7RnC9Qk
GTaLnLiF16OCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/
MGgGA1UdDgRhBF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1OTpjMjpmYTo0ZTo3
NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToyNDpiMDowNDpiMzpl
ODo5Nzo1Yjo3ZTBqBgNVHSMEYzBhgF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1
OTpjMjpmYTo0ZTo3NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToy
NDpiMDowNDpiMzplODo5Nzo1Yjo3ZTA/BgNVHREEODA2hjRzcGlmZmU6Ly8xMjRk
ZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIuY29uc3VsMD0GA1UdHgEB
/wQzMDGgLzAtgisxMjRkZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIu
Y29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIQDzkkI7R+0U12a+zq2EQhP/n2mHmta+
fs2hBxWIELGwTAIgLdO7RRw+z9nnxCIA6kNl//mIQb+PGItespiHZKAz74Q=
-----END CERTIFICATE-----
`
var invalidCA = `
-----BEGIN CERTIFICATE-----
not valid
-----END CERTIFICATE-----
`

var validAddress = "1.2.3.4:80"

var validServerName = "server.consul"

var validPeerID = "peer1"

// TODO(peering): the test methods below are exposed to prevent duplication,
// these should be removed at same time tests in peering_test get refactored.
// XXX: we can't put the existing tests in service_test.go into the peering
// package because it causes an import cycle by importing the top-level consul
// package (which correctly imports the agent/rpc/peering package)

// TestPeering is a test utility for generating a pbpeering.Peering with valid
// data along with the peerName, state and index.
func TestPeering(peerName string, state pbpeering.PeeringState) *pbpeering.Peering {
	return &pbpeering.Peering{
		Name:                peerName,
		PeerCAPems:          []string{validCA},
		PeerServerAddresses: []string{validAddress},
		PeerServerName:      validServerName,
		State:               state,
		// uncomment once #1613 lands
		// PeerID: validPeerID
	}
}

// TestPeeringToken is a test utility for generating a valid peering token
// with the given peerID for use in test cases
func TestPeeringToken(peerID string) structs.PeeringToken {
	return structs.PeeringToken{
		CA:              []string{validCA},
		ServerAddresses: []string{validAddress},
		ServerName:      validServerName,
		PeerID:          peerID,
	}
}

type mockClient struct {
	mu    sync.Mutex
	errCh chan error

	replicationStream *mockStream
}

func (c *mockClient) Send(r *pbpeering.ReplicationMessage) error {
	c.replicationStream.recvCh <- r
	return nil
}

func (c *mockClient) Recv() (*pbpeering.ReplicationMessage, error) {
	select {
	case err := <-c.errCh:
		return nil, err
	case r := <-c.replicationStream.sendCh:
		return r, nil
	case <-time.After(10 * time.Millisecond):
		return nil, io.EOF
	}
}

func (c *mockClient) RecvWithTimeout(dur time.Duration) (*pbpeering.ReplicationMessage, error) {
	select {
	case err := <-c.errCh:
		return nil, err
	case r := <-c.replicationStream.sendCh:
		return r, nil
	case <-time.After(dur):
		return nil, io.EOF
	}
}

func (c *mockClient) Close() {
	close(c.replicationStream.recvCh)
}

func newMockClient(ctx context.Context) *mockClient {
	return &mockClient{
		replicationStream: newTestReplicationStream(ctx),
	}
}

// mockStream mocks peering.PeeringService_StreamResourcesServer
type mockStream struct {
	sendCh chan *pbpeering.ReplicationMessage
	recvCh chan *pbpeering.ReplicationMessage

	ctx context.Context
	mu  sync.Mutex
}

var _ pbpeering.PeeringService_StreamResourcesServer = (*mockStream)(nil)

func newTestReplicationStream(ctx context.Context) *mockStream {
	return &mockStream{
		sendCh: make(chan *pbpeering.ReplicationMessage, 1),
		recvCh: make(chan *pbpeering.ReplicationMessage, 1),
		ctx:    ctx,
	}
}

// Send implements pbpeering.PeeringService_StreamResourcesServer
func (s *mockStream) Send(r *pbpeering.ReplicationMessage) error {
	s.sendCh <- r
	return nil
}

// Recv implements pbpeering.PeeringService_StreamResourcesServer
func (s *mockStream) Recv() (*pbpeering.ReplicationMessage, error) {
	r := <-s.recvCh
	if r == nil {
		return nil, io.EOF
	}
	return r, nil
}

// Context implements grpc.ServerStream and grpc.ClientStream
func (s *mockStream) Context() context.Context {
	return s.ctx
}

// SendMsg implements grpc.ServerStream and grpc.ClientStream
func (s *mockStream) SendMsg(m interface{}) error {
	return nil
}

// RecvMsg implements grpc.ServerStream and grpc.ClientStream
func (s *mockStream) RecvMsg(m interface{}) error {
	return nil
}

// SetHeader implements grpc.ServerStream
func (s *mockStream) SetHeader(metadata.MD) error {
	return nil
}

// SendHeader implements grpc.ServerStream
func (s *mockStream) SendHeader(metadata.MD) error {
	return nil
}

// SetTrailer implements grpc.ServerStream
func (s *mockStream) SetTrailer(metadata.MD) {}

type incrementalTime struct {
	base time.Time
	next uint64
}

func (t *incrementalTime) Now() time.Time {
	t.next++
	return t.base.Add(time.Duration(t.next) * time.Second)
}

func runStep(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}
