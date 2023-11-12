package internal

import "net"

type TCP4Config struct {
	Address         string `json:"address"`        // 地址
	MaxConnections  uint32 `json:"maxConnections"` // 最大连接数
	addressResolved *net.TCPAddr
}

type Config struct {
	Tcp4               []TCP4Config `json:"tcp4"`               // tcp4的配置
	MaxPendingMessages uint32       `json:"maxPendingMessages"` // 最大阻塞未发送的消息数
}

func parseServerConfig(config Config) (Config, error) {
	// 处理tcp4的配置
	tcp4Configs := config.Tcp4
	for index, tcp4Config := range config.Tcp4 {

		// 尝试解析监听地址
		address := tcp4Config.Address
		addr, err := net.ResolveTCPAddr("tcp4", address)
		if err != nil {
			return Config{}, err
		}

		tcp4Config.addressResolved = addr

		// 最大连接数
		if tcp4Config.MaxConnections <= 0 {
			tcp4Config.MaxConnections = 20000
		}

		tcp4Configs[index] = tcp4Config
	}
	config.Tcp4 = tcp4Configs

	// 最大阻塞未发送的消息数
	if config.MaxPendingMessages <= 0 {
		config.MaxPendingMessages = 1024
	}
	return config, nil
}
