package kernel_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/cpu/mmu"
	"github.com/sisoputnfrba/tp-golang/kernel/globals"
	"github.com/sisoputnfrba/tp-golang/utils/device"
	"github.com/sisoputnfrba/tp-golang/utils/pcb"
	"github.com/sisoputnfrba/tp-golang/utils/slice"
)

/* Glossary:

- BRQ: Body Request
- BRS: Body Response

*/

type ProcessStart_BRQ struct {
	Path string `json:"path"`
}

type ProcessStart_BRS struct {
	PID uint32 `json:"pid"`
}

type GetInstructions_BRQ struct {
	Path string `json:"path"`
	Pid  uint32 `json:"pid"`
	Pc 	uint32  `json:"pc"`
}

/**
 * ProcessInit: Inicia un proceso en base a un archivo dentro del FS de Linux.
	[x] Creación de PCB
	[x] Asignación de PID incrementando en 1 por cada proceso creado
	[x] Estado de proceso: NEW
*/
func ProcessInit(w http.ResponseWriter, r *http.Request) {
	var request ProcessStart_BRQ
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pathInst, err := json.Marshal(fmt.Sprintf(request.Path))
    if err != nil {
        http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
        return
    }
	pathInstString := string(pathInst)
	

	newPcb := &pcb.T_PCB{
		PID: 			generatePID(),
		PC: 			0,
		Quantum: 		uint32(globals.Configkernel.Quantum),
		CPU_reg: 		map[string]interface{}{
							"AX": uint8(0),
							"BX": uint8(0),
							"CX": uint8(0),
							"DX": uint8(0),
							"EAX": uint32(0),
							"EBX": uint32(0),
							"ECX": uint32(0),
							"EDX": uint32(0),
							"SI": uint32(0),
							"DI": uint32(0),
						},
		State: 			"NEW",
		EvictionReason: "",
		Resources: 		make(map[string]int),	// * El valor por defecto es 0, tener en cuenta por las dudas a la hora de testear
		RequestedResource: "",
	}

	globals.LTSMutex.Lock()
	slice.Push(&globals.LTS, *newPcb)
	defer globals.LTSMutex.Unlock()

	// Si la lista estaba vacía, la desbloqueo
	if len(globals.LTS) == 1 {
		globals.EmptiedListMutex.Unlock()
	}

	var respBody ProcessStart_BRS = ProcessStart_BRS{PID: newPcb.PID}

	response, err := json.Marshal(respBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Obtengo las instrucciones del proceso
	url := fmt.Sprintf("http://%s:%d/instrucciones", globals.Configkernel.IP_memory, globals.Configkernel.Port_memory)

	bodyInst, err := json.Marshal(GetInstructions_BRQ{
		Path: pathInstString,
		Pid: newPcb.PID,
		Pc: newPcb.PC,
	})
	if err != nil {
		return
	}
	
	requerirInstrucciones, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyInst))
	if err != nil {
		log.Printf("POST request failed (No se pueden cargar instrucciones): %v", err)
	}
	
	cliente := &http.Client{}
	requerirInstrucciones.Header.Set("Content-Type", "application/json")
	recibirRespuestaInstrucciones, err := cliente.Do(requerirInstrucciones)
	if (err != nil || recibirRespuestaInstrucciones.StatusCode != http.StatusOK) {
		log.Fatal("Error en CargarInstrucciones (memoria)", err)
	}

	log.Printf("Se crea el proceso %d en %s\n", newPcb.PID, newPcb.State)

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func generatePID() uint32 {
	globals.PidMutex.Lock()
	defer globals.PidMutex.Unlock()
	globals.NextPID++
	return globals.NextPID
}

/**
 * ProcessDelete: Elimina un proceso en base a un PID. Realiza las operaciones como si el proceso llegase a EXIT
	[ ] Cambio de estado de proceso: EXIT
	[ ] Liberación de recursos
	[ ] Liberación de archivos
	[ ] Liberación de memoria 
*/
func ProcessDelete(w http.ResponseWriter, r *http.Request) {
	pidString := r.PathValue("pid")
	pid, err := GetPIDFromString(pidString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Elimino el proceso de la lista de procesos
	RemoveByID(pid)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Job deleted")) // ! No tiene que devolver nada
}

type ProcessStatus_BRS struct {
	State string `json:"state"`
}

/**
 * ProcessState: Devuelve el estado de un proceso en base a un PID
	[ ] Devuelve el estado del proceso

	Por el momento devuelve un dato hardcodeado
*/
func ProcessState(w http.ResponseWriter, r *http.Request) {
	pidString := r.PathValue("pid")
	pid, err := GetPIDFromString(pidString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	process, _ := SearchByID(pid, globals.LTS)
	if process == nil {
		process, _ = SearchByID(pid, globals.STS)
	} 
	
	if process == nil {
		http.Error(w, "Process not found", http.StatusNotFound)
		return
	}

	result := ProcessStatus_BRS{State: process.State}

	response, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

/**
 * PlanificationStart: Retoma el STS y LTS en caso de que la planificación se encuentre pausada. Si no, ignora la petición.
*/
func PlanificationStart(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	<- globals.PlanBinary
}

/**
 * PlanificationStop: Detiene el STS y LTS en caso de que la planificación se encuentre en ejecución. Si no, ignora la petición.
	El proceso que se encuentra en ejecución NO es desalojado. Una vez que salga de EXEC se pausa el manejo de su motivo de desalojo.
	El resto de procesos bloqueados van a pausar su transición a la cola de Ready
*/
func PlanificationStop(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	globals.PlanBinary <- false
}

type ProcessList_BRS struct {
	Pid int `json:"pid"`
	State string `json:"state"`
}

/**
 * ProcessList: Devuelve una lista de procesos con su PID y estado
*/
func ProcessList(w http.ResponseWriter, r *http.Request) {
	allProcesses := getProcessList()

	// Formateo los procesos para devolverlos
	respBody := make([]ProcessList_BRS, len(allProcesses))
	for i, process := range allProcesses {
		respBody[i] = ProcessList_BRS{Pid: int(process.PID), State: process.State}
	}

	response, err := json.Marshal(respBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

/**
 * getProcessList: Devuelve una lista de todos los procesos en el sistema (LTS, STS, Blocked, STS_Priority, CurrentJob)

 * @return []pcb.T_PCB: Lista de procesos
*/
func getProcessList() []pcb.T_PCB {
	allProcesses := append(globals.LTS, globals.STS...)
	allProcesses = append(allProcesses, globals.Blocked...)
	allProcesses = append(allProcesses, globals.STS_Priority...)
	allProcesses = append(allProcesses, globals.CurrentJob)
	return allProcesses
}

/**
 * PCB_Send: Envía un PCB al CPU y recibe la respuesta

 * @return error: Error en caso de que falle el envío
*/
func PCB_Send() error {
	//Encode data
	jsonData, err := json.Marshal(globals.CurrentJob) // ? Semaforo?
	if err != nil {
		return fmt.Errorf("failed to encode PCB: %v", err)
	}

	client := &http.Client{
		Timeout: 0,
	}

	// Send data
	url := fmt.Sprintf("http://%s:%d/dispatch", globals.Configkernel.IP_cpu, globals.Configkernel.Port_cpu)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("POST request failed. Failed to send PCB: %v", err)
	}

	// Wait for response
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}

	// Decode response and update value
	err = json.NewDecoder(resp.Body).Decode(&globals.CurrentJob) // ? Semaforo?
	if err != nil {
		return fmt.Errorf("failed to decode PCB response: %v", err)
	}

	globals.PcbReceived <- true

	return nil
}

func PCB_recv(w http.ResponseWriter, r *http.Request) {
	var received_pcb pcb.T_PCB

	// Decode PCB
	err := json.NewDecoder(r.Body).Decode(&received_pcb)
	if err != nil {
		http.Error(w, "Failed to decode PCB", http.StatusBadRequest)
		return
	}
		
	globals.CurrentJob = received_pcb
	globals.PcbReceived <- true

	// Encode PCB
	jsonResp, err := json.Marshal(received_pcb)
	if err != nil {
		http.Error((w), "Failed to encode PCB response", http.StatusInternalServerError)
	}

	// Send back PCB
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)	
}

/**
 * SearchByID: Busca un proceso en la lista de procesos en base a su PID

 * @param pid: PID del proceso a buscar
 * @param processList: Lista de procesos
 * @return *pcb.T_PCB: Proceso encontrado
*/
func SearchByID(pid uint32, processList []pcb.T_PCB) (*pcb.T_PCB, int) {
	for i, process := range processList {
		if process.PID == pid {
			return &process, i
		}
	}
	return nil, -1
}

/**
 * RemoveByID: Remueve un proceso de la lista de procesos en base a su PID

 * @param pid: PID del proceso a remover
*/
func RemoveByID(pid uint32) error {
	_, ltsIndex := SearchByID(pid, globals.LTS)
	_, stsIndex := SearchByID(pid, globals.STS)
	
	if ltsIndex != -1 {
		globals.LTSMutex.Lock()
		defer globals.LTSMutex.Unlock()
		slice.RemoveAtIndex(&globals.LTS, ltsIndex)	
	} else if stsIndex != -1 {
		globals.STSMutex.Lock()
		defer globals.STSMutex.Unlock()
		slice.RemoveAtIndex(&globals.STS, stsIndex)
	}
	
	return nil
}

/**
 * GetPIDFromQueryPath: Convierte un PID en formato string a uint32

 * @param pidString: PID en formato string
 * @return uint32: PID extraído
*/
func GetPIDFromString(pidString string) (uint32, error) {
	pid64, error := strconv.ParseUint(pidString, 10, 32)
	return uint32(pid64), error
}

func RemoveFromBlocked(pid uint32) {
	for i, pcb := range globals.Blocked {
		if pcb.PID == pid {
			globals.MapMutex.Lock()
			defer globals.MapMutex.Unlock()
			slice.RemoveAtIndex(&globals.Blocked, i)
		}
	}

}

// ----------------- IO -----------------
func GetIOInterface(w http.ResponseWriter, r *http.Request) {
	var interf device.T_IOInterface
	
	err := json.NewDecoder(r.Body).Decode(&interf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newInterface := globals.InterfaceController{IoInterf: interf, Controller: make(chan bool, 1)}

	globals.Interfaces = append(globals.Interfaces, newInterface)

	log.Printf("Interface received, type: %s, port: %d\n", interf.InterfaceType, interf.InterfacePort)

	w.WriteHeader(http.StatusOK)
}
type SearchInterface struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
func ExisteInterfaz(w http.ResponseWriter, r *http.Request) {
	var received_data SearchInterface
	err := json.NewDecoder(r.Body).Decode(&received_data)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
	}
	
	log.Printf("Received data: %s, %s\n", received_data.Name, received_data.Type)

	aux, err := SearchDeviceByName(received_data.Name)
	if err != nil {
		http.Error(w, "Device not found", http.StatusNotFound)
	}
	
	var response globals.InterfaceController
	if aux.IoInterf.InterfaceType == received_data.Type {
		response = aux
	} else {
		http.Error(w, "Device type not match", http.StatusNotFound)
	}

	jsonResp, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}

func SearchDeviceByName(deviceName string) (globals.InterfaceController, error) {
	for _, interf := range globals.Interfaces {
		if interf.IoInterf.InterfaceName == deviceName  {
			fmt.Println("Interfaz encontrada: ", interf)
			return interf, nil
		}
	}
	return globals.InterfaceController{}, fmt.Errorf("device not found")
}

type Interfac_Time struct {
	Name 	string 	`json:"name"`
	WTime 	int 	`json:"wtime"`
}

var genIntTime Interfac_Time

func Resp_TiempoEspera (w http.ResponseWriter, r *http.Request) {
	var received_data Interfac_Time
	
	err := json.NewDecoder(r.Body).Decode(&received_data)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	//globals.ITimeSem.Lock()
	genIntTime = received_data

	w.WriteHeader(http.StatusOK)
}

// Me veo forzado a poner esta variable porque por alguna razón no puedo declarar una pcb dentro de la función
var genPCB pcb.T_PCB

type GenSleep struct {
	Pcb	 		pcb.T_PCB
	Inter 		device.T_IOInterface
	TimeToSleep int
}

func SolicitarGenSleep(pcb pcb.T_PCB) {
	newInter, err := SearchDeviceByName(genIntTime.Name)
	if err != nil {
		log.Printf("Device not found: %v", err)
	}
	
	newInter.Controller <- true

	genSleep := GenSleep{
		Pcb: pcb,
		Inter: newInter.IoInterf, 
		TimeToSleep: genIntTime.WTime,
	}

	globals.EnganiaPichangaMutex.Unlock()
	
	jsonData, err := json.Marshal(genSleep)
	if err != nil {
		log.Printf("Failed to encode PCB: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d/io-gen-sleep", newInter.IoInterf.InterfaceIP, newInter.IoInterf.InterfacePort)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to send PCB: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Unexpected response status: %s", resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&genPCB)
	if err != nil {
		log.Printf("Failed to decode PCB response: %v", err)
	}

	RemoveFromBlocked(genPCB.PID)
	genPCB.State = "READY"
	slice.Push(&globals.STS, genPCB)
	globals.MultiprogrammingCounter <- 1
	<- newInter.Controller
}

func IOStdinRead(w http.ResponseWriter, r *http.Request) {
	var infoRecibida struct {
		direccionesFisicas []mmu.DireccionTamanio
		interfaz globals.InterfaceController
		tamanio int
	}
	
	err := json.NewDecoder(r.Body).Decode(&infoRecibida)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Da la orden a la interfaz STDIN de leer			
	url := fmt.Sprintf("http://%s:%d/io-stdin-read", infoRecibida.interfaz.IoInterf.InterfaceIP, infoRecibida.interfaz.IoInterf.InterfacePort)

	bodyStdin, err := json.Marshal(struct {
		direccionesFisicas []mmu.DireccionTamanio
		tamanio int
	} {infoRecibida.direccionesFisicas, infoRecibida.tamanio})
	if err != nil {
		log.Printf("Failed to encode adresses: %v", err)
	}

	response, err := http.Post(url, "application/json", bytes.NewBuffer(bodyStdin))
	if err != nil {
		log.Printf("Failed to send adresses: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		log.Printf("Unexpected response status: %s", response.Status)
	}

	log.Printf("Kernel mandó a leer a la interfaz: ", infoRecibida.interfaz.InterfaceType, infoRecibida.interfaz.InterfacePort)

	globals.AvailablePcb <- true // TODO: Chequear si con la nueva implementacion se delega a la lista de bloqueados
	w.WriteHeader(http.StatusOK)
}

func IOStdoutWrite(w http.ResponseWriter, r *http.Request) {
	var infoRecibida struct {
		direccionesFisicas []mmu.DireccionTamanio
		interfaz globals.InterfaceController
	}
	
	err := json.NewDecoder(r.Body).Decode(&infoRecibida)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Da la orden a la interfaz STDOUT de mostrar por pantalla la salida			
	url := fmt.Sprintf("http://%s:%d/io-stdout-write", infoRecibida.interfaz.IoInterf.InterfaceIP, infoRecibida.interfaz.IoInterf.InterfacePort)

	bodyStdout, err := json.Marshal(infoRecibida.direccionesFisicas)
	if err != nil {
		log.Printf("Failed to encode adresses: %v", err)
	}

	response, err := http.Post(url, "application/json", bytes.NewBuffer(bodyStdout))
	if err != nil {
		log.Printf("Failed to send adresses: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		log.Printf("Unexpected response status: %s", response.Status)
	}

	log.Printf("Kernel mandó a escribir a la interfaz: ", infoRecibida.interfaz.InterfaceType, infoRecibida.interfaz.InterfacePort)

	globals.AvailablePcb <- true
	w.WriteHeader(http.StatusOK)
}