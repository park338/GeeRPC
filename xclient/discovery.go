package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

const (
	RandomSelect     SelectMode = iota // 随机选择
	RoundRobinSelect                   //轮询策略
)

type Discovery interface {
	Refresh() error                            //更新服务列表
	Update(servers []string) error             //手动更新列表
	Get(selectMode SelectMode) (string, error) // 拉取单个服务
	GetAll() ([]string, error)                 // 拉取全部服务
}

type MultiServersDiscovery struct {
	r       *rand.Rand //随机数
	mu      sync.RWMutex
	servers []string
	index   int //负载均衡策略
}

// 服务发现功能服务初始化
func NewMultiserversDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

var _ Discovery = (*MultiServersDiscovery)(nil)

func (m MultiServersDiscovery) Refresh() error {
	return nil
}

func (m MultiServersDiscovery) Update(servers []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
	return nil
}

func (m MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.servers)
	if n == 0 {
		return "", errors.New("没有找到任何服务")
	}
	switch mode {
	case RandomSelect:
		return m.servers[m.r.Intn(n)], nil
	case RoundRobinSelect:
		s := m.servers[m.index%n]
		m.index = (m.index + 1) % n
		return s, nil
	default:
		return "", errors.New("不支持所选模式")
	}
}

func (m MultiServersDiscovery) GetAll() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	servers := make([]string, len(m.servers), len(m.servers))
	copy(servers, m.servers)
	return servers, nil
}
