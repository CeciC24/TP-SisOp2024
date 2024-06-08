package memoria_api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/sisoputnfrba/tp-golang/memoria/globals"
)

type GetInstructions_BRQ struct {
	Path string `json:"path"`
	Pid  uint32 `json:"pid"`
	Pc   uint32 `json:"pc"`
}

type BitMap []int

func AbrirArchivo(filePath string) *os.File {
	file, err := os.Open(filePath) //El paquete nos provee el método ReadFile el cual recibe como argumento el nombre de un archivo el cual se encargará de leer. Al completar la lectura, retorna un slice de bytes, de forma que si se desea leer, tiene que ser convertido primero a una cadena de tipo string
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func InstruccionActual(w http.ResponseWriter, r *http.Request) {

	queryParams := r.URL.Query()
	pid := queryParams.Get("pid")
	pc := queryParams.Get("pc")
		
	respuesta, err := json.Marshal((BuscarInstruccionMap(PasarAInt(pc), PasarAInt(pid))))
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}
	
	log.Printf("La instruccion buscada fue: %s", BuscarInstruccionMap(PasarAInt(pc), PasarAInt(pid)))

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

func BuscarInstruccionMap(pc int, pid int) string {
	resultado := globals.InstruccionesProceso[pid][pc]
	return resultado
}

func PasarAInt(cadena string) int {
	num, err := strconv.Atoi(cadena)
	if err != nil {
		fmt.Println("Error: ", err)
	}
	return num
}

func CargarInstrucciones(w http.ResponseWriter, r *http.Request) {
	var request GetInstructions_BRQ
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pathInstrucciones := strings.Trim(request.Path, "\"")
	pid := request.Pid
	pc := request.Pc
	
	var instrucciones []string
	//Lee linea por linea el archivo
	file := AbrirArchivo(globals.Configmemory.Instructions_path + pathInstrucciones)
    defer file.Close()

    scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Agregar cada línea al slice de strings
		instrucciones = append(instrucciones, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
		
	globals.InstructionsMutex.Lock()
	defer globals.InstructionsMutex.Unlock()
	globals.InstruccionesProceso[int(pid)] = instrucciones
	log.Printf("Instrucciones cargadas para el PID %d ", pid)

	//acá debemos inicializar vacía la tabla de páginas para el proceso
	globals.Tablas_de_paginas[int(pid)] = []globals.Frame{}
	log.Printf("Tabla cargada para el PID %d ", pid)

	respuesta, err := json.Marshal((BuscarInstruccionMap(int(pc), int(pid))))
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

//--------------------------------------------------------------------------------------//
//RESIZE DE MEMORIA //CPU LE HACE LA PETICION DESDE estoVaParaCpuApi.go
func Resize(w http.ResponseWriter, r *http.Request) { //hay que hacer un patch ya que vamos a estar modificando un recurso existente (la tabla de páginas)
	queryParams := r.URL.Query()
	tamaño := queryParams.Get("tamaño")
	pid := queryParams.Get("pid")
	respuesta, err := json.Marshal(RealizarResize(PasarAInt(tamaño), PasarAInt(pid))) //devolver error out of memory
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}
//TODO: Verificar si "out of memory" se representa de esta forma

func RealizarResize(tamaño int, pid int) error {
cantPaginasActual := len(globals.Tablas_de_paginas[int(pid)])
	//ver cuantas paginas tiene el proceso en la tabla
cantPaginas := tamaño / globals.Configmemory.Page_size
//agregar a la tabla de páginas del proceso la cantidad de páginas que se le asignaron
globals.Tablas_de_paginas[int(pid)] = make(globals.TablaPaginas, cantPaginas)
/*make(globals.TablaPaginas, cantPaginas) crea una nueva tabla de páginas con una cantidad específica de páginas (cantPaginas).
Cada página en la tabla es un Frame.*/
ModificarTamañoProceso(cantPaginasActual, cantPaginas, pid)
log.Printf("Tabla de páginas del PID %d redimensionada a %d páginas", pid, cantPaginas)
return nil
}

func ModificarTamañoProceso(tamañoProcesoActual int, tamañoProcesoNuevo int, pid int) {
	if tamañoProcesoActual < tamañoProcesoNuevo { //ampliar proceso
		var diferenciaEnPaginas = tamañoProcesoNuevo - tamañoProcesoActual
		AmpliarProceso(diferenciaEnPaginas, pid)

	} else { // reducir proceso
		var diferenciaEnPaginas = tamañoProcesoActual - tamañoProcesoNuevo
		ReducirProceso(diferenciaEnPaginas, pid)
	}	
}

func AmpliarProceso(diferenciaEnPaginas int, pid int) error {
	for pagina := 0; pagina < diferenciaEnPaginas; pagina++ {
		marcoDisponible := false
		for i := 0; i < globals.Frames; i++ { //out of memory si no hay marcos disponibles
			if IsNotSet(i){
				//setear el valor del marco en la tabla de páginas del proceso
				globals.Tablas_de_paginas[pid][pagina] = globals.Frame(i)
				//marcar marco como ocupado
				Set(i)
				marcoDisponible = true
				// Salir del bucle una vez que se ha asignado un marco a la página
				break
			} 			
		} 
		if !marcoDisponible {
            return errors.New("out of memory")
        }
	}
	return nil
}

func ReducirProceso(diferenciaEnPaginas int, pid int){
	for  (diferenciaEnPaginas > 0){
		//obtener el marco que le corresponde a la página
		marco := BuscarMarco(pid, diferenciaEnPaginas)
		//marcar marco como desocupado
		Clear(marco)
		diferenciaEnPaginas--
	}
}
//--------------------------------------------------------------------------------------//
//ENVIAR MARCO A CPU
//Busca el marco que pertenece al proceso y a la página que envía CPU, dentro del diccionario
func BuscarMarco(pid int, pagina int) int {
	resultado := globals.Tablas_de_paginas[pid][pagina]
	return int(resultado)
	}

func EnviarMarco(w http.ResponseWriter, r *http.Request){
	//Ante cada peticion de CPU, dado un pid y una página, enviar frame a CPU
	queryParams := r.URL.Query()
	pid := queryParams.Get("pid")
	direccionLogica := queryParams.Get("direccionLogica")
	pagina := PasarAInt(direccionLogica) / globals.Configmemory.Page_size
	respuesta, err := json.Marshal((BuscarMarco(PasarAInt(pid), pagina)))
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}
//--------------------------------------------------------------------------------------//
//FINALIZACION DE PROCESO: PETICION DESDE KERNEL

func FinalizarProceso(pid int) {



}




//--------------------------------------------------------------------------------------//

// revisar si es necesario que sea slice de bytes
func NewBitMap(size int) BitMap {
	NewBMAp := make(BitMap,size)
	for i := 0; i < size; i++ {
		NewBMAp[i] = 0
	}
    return NewBMAp
}

func Set(i int) {
	globals.CurrentBitMap[i] = 1
}

func Clear(i int) {
	globals.CurrentBitMap[i] = 0
}

func IsNotSet(i int) bool {
  	return  globals.CurrentBitMap[i] == 0
}