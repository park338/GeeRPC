package xclient

import (
	"net/http"
	"strings"
	"time"
)

type GeeRegistryDiscovery struct {
	*MultiServersDiscovery
	registry   string
	timeout    time.Duration
	lastUpdate time.Time
}

const defaultUpdateTimeout = time.Second * 10

// 初始化服务发现
func NewGeeRegistryDiscovery(registryAddr string, timeout time.Duration) *GeeRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	d := &GeeRegistryDiscovery{
		MultiServersDiscovery: NewMultiserversDiscovery(make([]string, 0)),
		registry:              registryAddr,
		timeout:               timeout,
	}
	return d
}

// 手动更新服务
func (d *GeeRegistryDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	d.lastUpdate = time.Now()
	return nil
}

// 更新服务
func (d *GeeRegistryDiscovery) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lastUpdate.Add(d.timeout).After(time.Now()) {
		return nil
	}
	resp, err := http.Get(d.registry)
	if err != nil {
		return err
	}
	servers := strings.Split(resp.Header.Get("X-Geerpc-Servers"), ",")
	d.servers = make([]string, len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			d.servers = append(d.servers, strings.TrimSpace(server))
		}
	}
	d.lastUpdate = time.Now()
	return nil
}

// 获取服务
func (d *GeeRegistryDiscovery) Get(mode SelectMode) (string, error) {
	if err := d.Refresh(); err != nil {
		return "", err
	}
	return d.MultiServersDiscovery.Get(mode)
}

func (d *GeeRegistryDiscovery) GetAll() ([]string, error) {
	if err := d.Refresh(); err != nil {
		return nil, err
	}
	return d.MultiServersDiscovery.GetAll()
}
