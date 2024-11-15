package zitiedge

import (
	"gopkg.in/yaml.v3"
)

type ParseConfig interface {
	MarshalYAML() ([]byte, error)
	UnmarshalYAML([]byte) error
}

type Identity struct {
	Cert           string         `yaml:"cert"`
	ServerCert     string         `yaml:"server_cert"`
	Key            string         `yaml:"key"`
	Ca             string         `yaml:"ca"`
	AltServerCerts AltServerCerts `yaml:"alt_server_certs,omitempty"`
}

type AltServerCerts struct {
	ServerCert string `yaml:"server_cert"`
	ServerKey  string `yaml:"server_key"`
}

type Controller struct {
	Endpoint string `yaml:"endpoint"`
}

type Link struct {
	Dialers   []LinkDialer   `yaml:"dialers"`
	Listeners []LinkListener `yaml:"listeners,omitempty"`
}

type LinkDialer struct {
	Binding string `yaml:"binding"`
}

type LinkListener struct {
	Binding   string              `yaml:"binding"`
	Bind      string              `yaml:"bind"`
	Advertise string              `yaml:"advertise"`
	Options   LinkListenerOptions `yaml:"options"`
}

type LinkListenerOptions struct {
	OutQueueSize int `yaml:"outQueueSize"`
}

type EdgeListener struct {
	Binding string              `yaml:"binding"`
	Address string              `yaml:"address,omitempty"`
	Options EdgeListenerOptions `yaml:"options"`
}

type EdgeListenerOptions struct {
	Advertise         string `yaml:"advertise,omitempty"`
	ConnectTimeoutMs  int32  `yaml:"connectTimeoutMs,omitempty"`
	GetSessionTimeout int32  `yaml:"getSessionTimeout,omitempty"`
	Mode              string `yaml:"mode,omitempty"`
	Resolver          string `yaml:"resolver,omitempty"`
	LanIf             string `yaml:"lanIf,omitempty"`
	DnsSvcIpRange     string `yaml:"dnsSvcIpRange,omitempty"`
}

type CSR struct {
	Country            string `yaml:"country"`
	Province           string `yaml:"province"`
	Locality           string `yaml:"locality"`
	Organization       string `yaml:"organization"`
	OrganizationalUnit string `yaml:"organizationalUnit"`
	Sans               Sans   `yaml:"sans"`
}

type Sans struct {
	Dns []string `yaml:"dns"`
	Ip  []string `yaml:"ip"`
}

type Edge struct {
	CSR CSR `yaml:"csr"`
}

type Transport struct {
	Ws Ws `yaml:"ws"`
}

type Ws struct {
	WriteTimeout      string `yaml:"writeTimeout"`
	ReadTimeout       string `yaml:"readTimeout"`
	IdleTimeout       string `yaml:"idleTimeout"`
	PongTimeout       string `yaml:"pongTimeout"`
	PingInterval      string `yaml:"pingInterval"`
	HandshakeTimeout  string `yaml:"handshakeTimeout"`
	ReadBufferSize    string `yaml:"readBufferSize"`
	WriteBufferSize   string `yaml:"writeBufferSize"`
	EnableCompression string `yaml:"enableCompression"`
}

type Forwarder struct {
	ListatencyProbeInterval int32 `yaml:"latencyProbeInterval"`
	XgressDialQueueLength   int32 `yaml:"xgressDialQueueLength"`
	XgressDialWorkerCount   int32 `yaml:"xgressDialWorkerCount"`
	LinkDialQueueLength     int32 `yaml:"linkDialQueueLength"`
	LinkDialWorkerCount     int32 `yaml:"linkDialWorkerCount"`
	RateLimitedQueueLength  int32 `yaml:"rateLimitedQueueLength"`
	RateLimitedWorkerCount  int32 `yaml:"rateLimitedWorkerCount"`
}

type Router struct {
	Version    int            `yaml:"v"`
	Identity   Identity       `yaml:"identity"`
	Controller Controller     `yaml:"ctrl"`
	Link       Link           `yaml:"link"`
	Listeners  []EdgeListener `yaml:"listeners,omitempty"`
	CSR        CSR            `yaml:"csr,omitempty"`
	Edge       Edge           `yaml:"edge,omitempty"`
	Transport  Transport      `yaml:"transport,omitempty"`
	Forwarder  Forwarder      `yaml:"forwarder"`
}

func (r *Router) UnmarshalYAML(data []byte) error {
	return yaml.Unmarshal(data, r)
}

func (r *Router) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(r)

}

// func (r *Router) IsZero() bool {
// 	return reflect.ValueOf(r.Identity.AltServerCerts).IsZero() &&
// 		reflect.ValueOf(r.Link.Listeners).IsZero() &&
// 		reflect.ValueOf(r.CSR).IsZero() &&
// 		reflect.ValueOf(r.Transport).IsZero()

// }
