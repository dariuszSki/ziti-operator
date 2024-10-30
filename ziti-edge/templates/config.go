package zitiedge

import "gopkg.in/yaml.v2"

type Config interface {
	Update() string
}

type Router struct {
	Version  int `yaml:"v"`
	Identity struct {
		Cert           string `yaml:"cert"`
		ServerCert     string `yaml:"server_cert"`
		Key            string `yaml:"key"`
		Ca             string `yaml:"ca"`
		AltServerCerts struct {
			ServerCert string `yaml:"server_cert"`
			ServerKey  string `yaml:"server_key"`
		} `yaml:"alt_server_certs"`
	} `yaml:"identity"`
	Controller struct {
		Endpoint string `yaml:"endpoint"`
	} `yaml:"ctrl"`
	Link struct {
		Dialers []struct {
			Binding string `yaml:"binding"`
		} `yaml:"dialers"`
		Listeners []struct {
			Binding   string `yaml:"binding"`
			Bind      string `yaml:"bind"`
			Advertise string `yaml:"advertise"`
			Options   struct {
				OutQueueSize int `yaml:"outQueueSize"`
			} `yaml:"options"`
		} `yaml:"listeners,omitempty"`
	} `yaml:"link"`
	Listeners []struct {
		Binding string `yaml:"binding"`
		Address string `yaml:"address,omitempty"`
		Options struct {
			Advertise         string `yaml:"advertise,omitempty"`
			ConnectTimeoutMs  string `yaml:"connectTimeoutMs,omitempty"`
			GetSessionTimeout string `yaml:"getSessionTimeout,omitempty"`
			Mode              string `yaml:"mode,omitempty"`
			Resolver          string `yaml:"resolver,omitempty"`
			LanIf             string `yaml:"lanIf,omitempty"`
			DnsSvcIpRange     string `yaml:"dnsSvcIpRange,omitempty"`
		} `yaml:"options"`
	} `yaml:"listeners,omitempty"`
	CSR struct {
		Country            string `yaml:"country"`
		Province           string `yaml:"province"`
		Locality           string `yaml:"locality"`
		Organization       string `yaml:"organization"`
		OrganizationalUnit string `yaml:"organizationalUnit"`
		Sans               struct {
			Dns []string `yaml:"dns"`
			Ip  []string `yaml:"ip"`
		} `yaml:"sans"`
	} `yaml:"csr,omitempty"`
	Edge struct {
		CSR struct {
			Country            string `yaml:"country"`
			Province           string `yaml:"province"`
			Locality           string `yaml:"locality"`
			Organization       string `yaml:"organization"`
			OrganizationalUnit string `yaml:"organizationalUnit"`
			Sans               struct {
				Dns []string `yaml:"dns"`
				Ip  []string `yaml:"ip"`
			} `yaml:"sans"`
		} `yaml:"csr"`
	} `yaml:"edge,omitempty"`
	Transport struct {
		Ws struct {
			WriteTimeout      string `yaml:"writeTimeout"`
			ReadTimeout       string `yaml:"readTimeout"`
			IdleTimeout       string `yaml:"idleTimeout"`
			PongTimeout       string `yaml:"pongTimeout"`
			PingInterval      string `yaml:"pingInterval"`
			HandshakeTimeout  string `yaml:"handshakeTimeout"`
			ReadBufferSize    string `yaml:"readBufferSize"`
			WriteBufferSize   string `yaml:"writeBufferSize"`
			EnableCompression string `yaml:"enableCompression"`
		} `yaml:"ws"`
	} `yaml:"transport,omitempty"`
	Forwarder struct {
		ListatencyProbeInterval string `yaml:"latencyProbeInterval"`
		XgressDialQueueLength   string `yaml:"xgressDialQueueLength"`
		XgressDialWorkerCount   string `yaml:"xgressDialWorkerCount"`
		LinkDialQueueLength     string `yaml:"linkDialQueueLength"`
		LinkDialWorkerCount     string `yaml:"linkDialWorkerCount"`
	} `yaml:"forwarder"`
}

func (r Router) Update() string {
	byteConfig, _ := yaml.Marshal(&r)
	return string(byteConfig)
}
