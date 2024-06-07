package globals

type Config struct {
	Puerto                 int      `json:"port"`
	IpMemoria              string   `json:"mensaje"`
	PuertoMemoria          int      `json:"port_memory"`
	IpCPU                  string   `json:"ip_cpu"`
	PuertoCPU              int      `json:"port_cpu"`
	AlgoritmoPlanificacion string   `json:"planning_algorithm"`
	Quantum                int      `json:"quantum"`
	Recursos               []string `json:"resources"`
	InstanciasRecursos     []int    `json:"resource_instances"`
	Multiprogramacion      int      `json:"multiprogramacion"`
}

var ClientConfig *Config
