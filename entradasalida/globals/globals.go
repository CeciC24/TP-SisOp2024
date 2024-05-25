package globals

import "github.com/sisoputnfrba/tp-golang/utils/pcb"

type T_ConfigIO struct {
	Port               int    `json:"port"`
	Type               string `json:"type"`
	Unit_work_time     int    `json:"unit_work_time"`
	Ip_kernel          string `json:"ip_kernel"`
	Port_kernel        int    `json:"port_kernel"`
	Ip_memory          string `json:"ip_memory"`
	Port_memory        int    `json:"port_memory"`
	Dialfs_path        string `json:"dialfs_path"`
	Dialfs_block_size  int    `json:"dialfs_block_size"`
	Dialfs_block_count int    `json:"dialfs_block_count"`
}

var ConfigIO 		T_ConfigIO
var TiempoEspera 	int
var SleepPCB		pcb.T_PCB