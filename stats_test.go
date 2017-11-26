package stats

import (
	"net"
	"testing"
	"time"

	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	client "github.com/influxdata/influxdb/client/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	pingDefaultValue   = "Hello World."
	countListResponses = 20
)

func TestServerInterceptorSuite(t *testing.T) {
	suite.Run(t, &ServerInterceptorTestSuite{})
}

type ServerInterceptorTestSuite struct {
	suite.Suite

	serverListener net.Listener
	server         *grpc.Server
	clientConn     *grpc.ClientConn
	testClient     pb_testproto.TestServiceClient
	ctx            context.Context
	c              client.Client
}

func (s *ServerInterceptorTestSuite) SetupSuite() {
	var err error

	s.c, err = NewInfluxClient("")

	s.serverListener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(s.T(), err, "must be able to allocate a port for serverListener")

	// This is the point where we hook up the interceptor
	s.server = grpc.NewServer(
		//grpc.StreamInterceptor(StreamServerInterceptor),
		grpc.UnaryInterceptor(UnaryServerInterceptor(s.c, InfluxOptions{})),
	)
	pb_testproto.RegisterTestServiceServer(s.server, &testService{t: s.T()})

	go func() {
		s.server.Serve(s.serverListener)
	}()

	s.clientConn, err = grpc.Dial(s.serverListener.Addr().String(), grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(2*time.Second))
	require.NoError(s.T(), err, "must not error on client Dial")
	s.testClient = pb_testproto.NewTestServiceClient(s.clientConn)
}

func (s *ServerInterceptorTestSuite) SetupTest() {
	// Make all RPC calls last at most 2 sec, meaning all async issues or deadlock will not kill tests.
	s.ctx, _ = context.WithTimeout(context.TODO(), 2*time.Second)
}

func (s *ServerInterceptorTestSuite) TearDownSuite() {
	if s.serverListener != nil {
		s.server.Stop()
		s.T().Logf("stopped grpc.Server at: %v", s.serverListener.Addr().String())
		s.serverListener.Close()

	}
	if s.clientConn != nil {
		s.clientConn.Close()
	}
}

func (s *ServerInterceptorTestSuite) TestUnaryIncrementsStarted() {
	s.testClient.PingEmpty(s.ctx, &pb_testproto.Empty{})

	s.testClient.PingError(s.ctx, &pb_testproto.PingRequest{ErrorCodeReturned: uint32(codes.Unavailable)})
}

type testService struct {
	t *testing.T
}

func (s *testService) PingEmpty(ctx context.Context, _ *pb_testproto.Empty) (*pb_testproto.PingResponse, error) {
	return &pb_testproto.PingResponse{Value: pingDefaultValue, Counter: 42}, nil
}

func (s *testService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	// Send user trailers and headers.
	return &pb_testproto.PingResponse{Value: ping.Value, Counter: 42}, nil
}

func (s *testService) PingError(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.Empty, error) {
	code := codes.Code(ping.ErrorCodeReturned)
	return nil, grpc.Errorf(code, "Userspace error.")
}

func (s *testService) PingStream(ping pb_testproto.TestService_PingStreamServer) error {
	return nil
}

func (s *testService) PingList(ping *pb_testproto.PingRequest, stream pb_testproto.TestService_PingListServer) error {
	if ping.ErrorCodeReturned != 0 {
		return grpc.Errorf(codes.Code(ping.ErrorCodeReturned), "foobar")
	}
	// Send user trailers and headers.
	for i := 0; i < countListResponses; i++ {
		stream.Send(&pb_testproto.PingResponse{Value: ping.Value, Counter: int32(i)})
	}
	return nil
}

func TestNewClient(t *testing.T) {

	c, err := NewInfluxClient("")

	d, _ := time.ParseDuration("2s")

	ping, _, _ := c.Ping(d)

	// assert equality
	assert.Equal(t, ping, time.Duration(0), "they should be equal")

	// assert for nil (good for errors)
	assert.Nil(t, err)

	assert.NotNil(t, c)
}

func TestNewOptions(t *testing.T) {

	opts := NewOpts("myservice", "mymeasurement", "mydb")

	// assert equality
	assert.Equal(t, opts.Service, "myservice", "they should be equal")
	assert.Equal(t, opts.Measurement, "mymeasurement", "they should be equal")
	assert.Equal(t, opts.Database, "mydb", "they should be equal")

	assert.NotNil(t, opts)
}
