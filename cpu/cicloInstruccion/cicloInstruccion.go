package cicloInstruccion

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
	"github.com/sisoputnfrba/tp-golang/utils/pcb"
	"github.com/sisoputnfrba/tp-golang/utils/semaphores"
)

func Delimitador(instActual string) []string {
	delimitador := " "
	i := 0

	instruccionDecodificadaConComillas := strings.Split(instActual, delimitador)
	instruccionDecodificada := instruccionDecodificadaConComillas

	largoInstruccion := len (instruccionDecodificadaConComillas) 
	for i < largoInstruccion {
		instruccionDecodificada[i] = strings.Trim(instruccionDecodificadaConComillas[i], `"`)
		i++
	}
	
	return instruccionDecodificada
}

func Fetch(currentPCB *pcb.T_PCB) string {
	//CPU pasa a memoria el PID y el PC, y memoria le devuelve la instrucción
	//(después de identificar en el diccionario la key:PID,
	//va a buscar en la lista de instrucciones de ese proceso, la instrucción en la posición
	//pc y nos va a devolver esa instrucción)
	// GET /instrucciones	
	semaphores.PCBMutex.Lock()
	pid := currentPCB.PID
	pc := currentPCB.PC
	semaphores.PCBMutex.Unlock()
	
	cliente := &http.Client{}
	url := fmt.Sprintf("http://%s:%d/instrucciones", globals.Configcpu.IP_memory,globals.Configcpu.Port_memory)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Error al hacer el request")
	}
	q := req.URL.Query()
	q.Add("pid", strconv.Itoa(int(pid)))
	q.Add("pc", strconv.Itoa(int(pc)))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Content-Type", "application/json")
	respuesta, err := cliente.Do(req)
	if err != nil {
		log.Fatal("Error al hacer el request")
	}

	if respuesta.StatusCode != http.StatusOK {
		log.Fatal("Error en el estado de la respuesta")
	}

	instruccion, err := io.ReadAll(respuesta.Body)
	if err != nil {
		log.Fatal("Error al leer el cuerpo de la respuesta")
	}

	fmt.Println(string(instruccion))
	
	instruccion1 := string(instruccion)
		
	log.Printf("PID: %d - FETCH - Program Counter: %d", pid, pc)

	return instruccion1
}

func DecodeAndExecute(currentPCB *pcb.T_PCB) {
	instActual := Fetch(currentPCB)
	instruccionDecodificada := Delimitador(instActual)
	
	if (instruccionDecodificada[0] == "EXIT"){
		pcb.EvictionFlag = true
		currentPCB.EvictionReason = "EXIT"

		log.Printf("PID: %d - Ejecutando: %s", currentPCB.PID, instruccionDecodificada[0])
	} else {
		log.Printf("PID: %d - Ejecutando: %s - %s", currentPCB.PID, instruccionDecodificada[0], instruccionDecodificada[1:])
	}

	switch instruccionDecodificada[0] {
		case "IO_GEN_SLEEP": 
			//operaciones.IO_GEN_SLEEP(instruccionActual.parametro1, instruccionActual.parametro2)

		case "JNZ":
			if currentPCB.CPU_reg[instruccionDecodificada[1]] != 0 {
				currentPCB.PC = ConvertirUint32(instruccionDecodificada[2])
			} else {
				currentPCB.PC++
			}
			
		case "SET":
			tipoReg := pcb.TipoReg(instruccionDecodificada[1])
			valor := instruccionDecodificada[2]
			
			if (tipoReg == "uint32") {
				currentPCB.CPU_reg[instruccionDecodificada[1]] = ConvertirUint32(valor)
			} else {
				currentPCB.CPU_reg[instruccionDecodificada[1]] = ConvertirUint8(valor)
			}
			currentPCB.PC++
			
		case "SUM":
			tipoReg1 := pcb.TipoReg(instruccionDecodificada[1])
			valorReg2 := currentPCB.CPU_reg[instruccionDecodificada[2]]
			
			tipoActualReg1 := reflect.TypeOf(currentPCB.CPU_reg[instruccionDecodificada[1]]).String()
			tipoActualReg2 := reflect.TypeOf(valorReg2).String()

			if (tipoReg1 == "uint32") {
				currentPCB.CPU_reg[instruccionDecodificada[1]] = Convertir[uint32](tipoActualReg1, currentPCB.CPU_reg[instruccionDecodificada[1]]) + Convertir[uint32](tipoActualReg2, valorReg2)

			} else {
				currentPCB.CPU_reg[instruccionDecodificada[1]] = Convertir[uint8](tipoActualReg1, currentPCB.CPU_reg[instruccionDecodificada[1]]) + Convertir[uint8](tipoActualReg2, valorReg2)
			}
			currentPCB.PC++
				
		case "SUB":
			//SUB (Registro Destino, Registro Origen): Resta al Registro Destino 
			//el Registro Origen y deja el resultado en el Registro Destino.
			tipoReg1 := pcb.TipoReg(instruccionDecodificada[1])
			valorReg2 := currentPCB.CPU_reg[instruccionDecodificada[2]]
			
			tipoActualReg1 := reflect.TypeOf(currentPCB.CPU_reg[instruccionDecodificada[1]]).String()
			tipoActualReg2 := reflect.TypeOf(valorReg2).String()

			if (tipoReg1 == "uint32") {
				currentPCB.CPU_reg[instruccionDecodificada[1]] = Convertir[uint32](tipoActualReg1, currentPCB.CPU_reg[instruccionDecodificada[1]]) - Convertir[uint32](tipoActualReg2, valorReg2)

			} else {
				currentPCB.CPU_reg[instruccionDecodificada[1]] = Convertir[uint8](tipoActualReg1, currentPCB.CPU_reg[instruccionDecodificada[1]]) - Convertir[uint8](tipoActualReg2, valorReg2)
			}
			currentPCB.PC++
	}

}

type Uint interface {~uint8 | ~uint32}
func Convertir[T Uint](tipo string, parametro interface {}) T {

	if parametro == "" {
		log.Fatal("La cadena de texto está vacía")
	}

	switch tipo {
		case "uint8":
			valor := parametro.(uint8)
			return T(valor)
		case "uint32":
			valor := parametro.(uint32)
			return T(valor)
		case "float64":
			valor := parametro.(float64)
			return T(valor)
	}
	return T(0)
}

func ConvertirUint8(parametro string) uint8 {
	parametroConvertido, err := strconv.Atoi(parametro)
	if err != nil {
		log.Fatal("Error al convertir el parametro a uint8")
	}
	return uint8(parametroConvertido)
}

func ConvertirUint32(parametro string) uint32 {
	parametroConvertido, err := strconv.Atoi(parametro)
	if err != nil {
		log.Fatal("Error al convertir el parametro a uint32")
	}
	return uint32(parametroConvertido)
}
