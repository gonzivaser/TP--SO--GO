package globals

type Config struct {
	Puerto                 int      `json:"port"`
	IpMemoria              string   `json:"mensaje"`
	PuertoMemoria          int      `json:"puerto_memoria"`
	IpCPU                  string   `json:"ip_cpu"`
	PuertoCPU              int      `json:"puerto_cpu"`
	AlgoritmoPlanificacion string   `json:"algoritmo_planificacion"`
	Quantum                int      `json:"quantum"`
	Recursos               []string `json:"recursos"`
	InstanciasRecursos     []int    `json:"instancias_recursos"`
	Multiprogramacion      int      `json:"multiprogramacion"`
}

var ClientConfig *Config
