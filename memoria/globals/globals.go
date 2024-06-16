package globals

type Config struct {
	Puerto           int    `json:"port"`
	MemorySize       int    `json:"memory_size"`
	PuertoCPU        int    `json:"port_cpu"`
	PageSize         int    `json:"page_size"`
	InstructionsPath string `json:"instructions_path"`
	DelayResponse    int    `json:"delay_response"`
}

var ClientConfig *Config
