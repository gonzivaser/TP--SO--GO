package globals

type Config struct {
	Puerto                string `json:"puerto"`
	Tipo                  string `json:"tipo"`
	UnidadDeTiempo        int    `json:"unidad_de_tiempo"`
	IPKernel              string `json:"ip_kernel"`
	PuertoKernel          string `json:"puerto_kernel"`
	IPMemoria             string `json:"ip_memoria"`
	PuertoMemoria         string `json:"puerto_memoria"`
	PathDialFS            string `json:"path_dialfs"`
	TamanioBloqueDialFS   int    `json:"tamanio_bloque_dialfs"`
	CantidadBloquesDialFS int    `json:"cantidad_bloques_dialfs"`
}

var ClientConfig *Config

type Interfaces struct {
	Nombre string
	Config *Config
}
