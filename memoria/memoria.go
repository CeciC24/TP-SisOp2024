package main

import (
	"log"
	"net/http"

	memoria_api "github.com/sisoputnfrba/tp-golang/memoria/API"
	"github.com/sisoputnfrba/tp-golang/memoria/globals"
	cfg "github.com/sisoputnfrba/tp-golang/utils/config"
	logger "github.com/sisoputnfrba/tp-golang/utils/log"
	"github.com/sisoputnfrba/tp-golang/utils/server-Functions"
)

func main() {
	// Iniciar loggers
	logger.ConfigurarLogger("memory.log")
	logger.LogfileCreate("memory_debug.log")

	// Inicializamos la config
	err := cfg.ConfigInit("config-memory.json", &globals.Configmemory)
	if err != nil {
		log.Fatalf("Error al cargar la configuracion %v", err)
	}

	cfg.VEnvMemoria(nil, &globals.Configmemory.Port)

	log.Println("Configuracion MEMORIA cargada")

	globals.User_Memory = make([]byte, globals.Configmemory.Memory_size)
	//verificar si estan bien los punteros
	// Calculo la cantidad de frames que tendrá la memoria
	globals.Frames = globals.Configmemory.Memory_size / globals.Configmemory.Page_size //ver si hay que ponerle puntero

	globals.CurrentBitMap = memoria_api.NewBitMap(globals.Frames)

	// Handlers
	// Iniciar servidor

	// log.Println("Instrucciones leídas por memoria.")
	go server.ServerStart(globals.Configmemory.Port, RegisteredModuleRoutes())
	// log.Println("Instrucciones enviadas a CPU")

	select {}

}

func RegisteredModuleRoutes() http.Handler {
	moduleHandler := &server.ModuleHandler{
		RouteHandlers: map[string]http.HandlerFunc{
			"GET /instrucciones":      memoria_api.InstruccionActual,
			"POST /instrucciones":     memoria_api.CargarInstrucciones,
			"GET /enviarMarco":        memoria_api.EnviarMarco,      //implementada en la MMU
			"PATCH /resize":           memoria_api.Resize,           //implementada en CPU
			"PATCH /finalizarProceso": memoria_api.FinalizarProceso, //falta implementar desde KERNEL
			"POST /read":              memoria_api.LeerMemoria,      // implementada en cpu
			"POST /write":             memoria_api.EscribirMemoria, // implementada en cpu
			"GET /tamPagina":          memoria_api.Page_size,
			"GET /tamTabla":           memoria_api.PedirTamTablaPaginas,        //falta implementar desde cliente
			"GET /delay":			   memoria_api.SendDelay,                    //falta implementar desde cliente
		},
	}
	return moduleHandler
}
