package operaciones

import (
	"log"
)

type T_Registers struct {
	PC  uint32 `json:"pc"`
	SI  uint32 `json:"si"`
	DI  uint32 `json:"di"`
}

func IO_GEN_SLEEP(cantidadUnidadesTrabajo int, cantTiempoDeTrabajo int) {
	
}

// para llamarla SET(&registro, valor)
func SET(registro *uint32, valor uint32) {
	*registro = valor
}

// para llamarla SUM(&registroDestino, registroOrigen)
func SUM(registroDestino *uint32, registroOrigen uint32) {
	*registroDestino = *registroDestino + registroOrigen
}

// para llamarla SUB(&registroDestino, registroOrigen)
func SUB(registroDestino *uint32, registroOrigen uint32) {
	if *registroDestino  >= registroOrigen {
		*registroDestino = *registroDestino - registroOrigen
	}
}


func JNZ(registro uint32,  parametro uint32) {//instruccion pcb.T_PCB) { //A QUE SE REFIERE CON INSTRUCCIOOOOOON
	if 	registro != 0 {
		log.Printf("El PC de la instruccion actual es %d", parametro)
	}
	
}
