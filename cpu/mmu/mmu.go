package mmu

import (
	//"encoding/json"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
	"github.com/sisoputnfrba/tp-golang/utils/pcb"
)

func Frame_rcv(currentPCB *pcb.T_PCB, direccionLogica int) int {
	//Enviamos el PID y la PAGINA a memoria
	pid := currentPCB.PID

	cliente := &http.Client{}
	url := fmt.Sprintf("http://%s:%d/enviarMarco", globals.Configcpu.IP_memory,globals.Configcpu.Port_memory)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Error al hacer el request")
	}
	q := req.URL.Query()
	q.Add("pid", strconv.Itoa(int(pid)))
	q.Add("page", strconv.Itoa(direccionLogica)) //paso la direccionLogica completa y no la página porque quien tiene el tamaño de la página es memoria
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Content-Type", "application/json")
	respuesta, err := cliente.Do(req)
	if err != nil {
		log.Fatal("Error al hacer el request")
	}

	if respuesta.StatusCode != http.StatusOK {
		log.Fatal("Error en el estado de la respuesta")
	}

	//Memoria nos devuelve un frame a partir de la data enviada
	frame, err := io.ReadAll(respuesta.Body)
	if err != nil {
		log.Fatal("Error al leer el cuerpo de la respuesta")
	}

	return int(bytesToInt(frame))
}

func ObtenerDireccionFisica(direccionLogica int) int { //ver de donde sale el n de pag y el desplazamiento
	numeroPagina := direccionLogica/Frame_rcv(&globals.CurrentJob, direccionLogica)
	desplazamiento := direccionLogica - numeroPagina*Frame_rcv(&globals.CurrentJob, direccionLogica)
	direccionFisica := Frame_rcv(&globals.CurrentJob, direccionLogica)*numeroPagina + desplazamiento

	return direccionFisica //devuelve la direccion fisica, revisar cómo tiene que interpretarla memoria, como un int?
}

func bytesToInt(b []byte) uint32 {
    return binary.BigEndian.Uint32(b)
}