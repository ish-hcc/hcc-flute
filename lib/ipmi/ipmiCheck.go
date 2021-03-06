package ipmi

import (
	"hcc/flute/lib/config"
	"hcc/flute/lib/logger"
	"hcc/flute/lib/mysql"
	"hcc/flute/model"
	"strconv"
	"time"
)

var checkAllLocked = false
var checkStatusLocked = false
var checkNodesDetailLocked = false

func delayMillisecond(n time.Duration) {
	time.Sleep(n * time.Millisecond)
}

func checkAllLock() {
	checkAllLocked = true
}

func checkAllUnlock() {
	checkAllLocked = false
}

func checkStatusLock() {
	checkStatusLocked = true
}

func checkStatusUnlock() {
	checkStatusLocked = false
}

func checkNodesDetailLock() {
	checkNodesDetailLocked = true
}

func checkNodesDetailUnlock() {
	checkNodesDetailLocked = false
}

// UpdateAllNodes : Get all infos from IPMI nodes and update database (except power state)
func UpdateAllNodes() (interface{}, error) {
	var nodes []model.Node
	var bmcIP string

	sql := "select bmc_ip from node where active = 1"
	stmt, err := mysql.Db.Query(sql)
	if err != nil {
		logger.Logger.Println(err)
		return nil, nil
	}
	defer func() {
		_ = stmt.Close()
	}()

	for stmt.Next() {
		err := stmt.Scan(&bmcIP)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		serialNo, err := GetSerialNo(bmcIP)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		uuid, err := GetUUID(bmcIP, serialNo)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		bmcMAC, err := GetNICMac(bmcIP, int(config.Ipmi.BaseboardNICNoBMC), true)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		pxeMAC, err := GetNICMac(bmcIP, int(config.Ipmi.BaseboardNICNoPXE), false)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		processors, err := GetProcessors(bmcIP, serialNo)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		cpuCores, err := GetProcessorsCores(bmcIP, serialNo, processors)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		memory, err := GetTotalSystemMemory(bmcIP, serialNo)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		node := model.Node{
			UUID:       uuid,
			BmcMacAddr: bmcMAC,
			BmcIP:      bmcIP,
			PXEMacAddr: pxeMAC,
			CPUCores:   cpuCores,
			Memory:     memory,
		}

		sql := "update node set bmc_mac_addr = ?, pxe_mac_addr = ?, cpu_cores = ?, memory = ? where uuid = ?"
		stmt, err := mysql.Db.Prepare(sql)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		result, err2 := stmt.Exec(node.BmcMacAddr, node.PXEMacAddr, node.CPUCores, node.Memory, node.UUID)
		if err2 != nil {
			logger.Logger.Println(err2)
			_ = stmt.Close()
			continue
		}
		_ = stmt.Close()

		if config.Ipmi.Debug == "on" {
			logger.Logger.Println(result.LastInsertId())
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// UpdateStatusNodes : Get status from IPMI nodes and update database
func UpdateStatusNodes() (interface{}, error) {
	var nodes []model.Node
	var uuid string
	var bmcIP string

	sql := "select uuid, bmc_ip from node where active = 1"
	stmt, err := mysql.Db.Query(sql)
	if err != nil {
		logger.Logger.Println(err)
		return nil, nil
	}
	defer func() {
		_ = stmt.Close()
	}()

	for stmt.Next() {
		err := stmt.Scan(&uuid, &bmcIP)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		serialNo, err := GetSerialNo(bmcIP)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		powerState, err := GetPowerState(bmcIP, serialNo)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		node := model.Node{
			UUID:   uuid,
			Status: powerState,
		}

		sql = "update node set status = ? where uuid = ?"
		stmt, err := mysql.Db.Prepare(sql)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		result, err2 := stmt.Exec(node.Status, node.UUID)
		if err2 != nil {
			logger.Logger.Println(err2)
			_ = stmt.Close()
			continue
		}
		_ = stmt.Close()

		if config.Ipmi.Debug == "on" {
			logger.Logger.Println(result.LastInsertId())
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// UpdateNodesDetail : Get detail infos from IPMI nodes and update database
func UpdateNodesDetail() (interface{}, error) {
	var nodedetails []model.NodeDetail
	var uuid string
	var bmcIP string

	sql := "select uuid, bmc_ip from node where active = 1"
	stmt, err := mysql.Db.Query(sql)
	if err != nil {
		logger.Logger.Println(err)
		return nil, nil
	}
	defer func() {
		_ = stmt.Close()
	}()

	for stmt.Next() {
		err := stmt.Scan(&uuid, &bmcIP)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		serialNo, err := GetSerialNo(bmcIP)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		processorModel, err := GetProcessorModel(bmcIP, serialNo)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		processors, err := GetProcessors(bmcIP, serialNo)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		threads, err := GetProcessorsThreads(bmcIP, serialNo, processors)
		if err != nil {
			logger.Logger.Println(err)
			continue
		}

		nodedetail := model.NodeDetail{
			NodeUUID:      uuid,
			CPUModel:      processorModel,
			CPUProcessors: processors,
			CPUThreads:    threads,
		}

		sql := "select node_uuid from node_detail where node_uuid = ?"
		err = mysql.Db.QueryRow(sql, uuid).Scan(&uuid)
		if err != nil {
			logger.Logger.Println("Inserting not existing new node_detail")

			sql = "insert into node_detail(node_uuid, cpu_model, cpu_processors, cpu_threads) values (?, ?, ?, ?)"
			stmt, err := mysql.Db.Prepare(sql)
			if err != nil {
				logger.Logger.Println(err)
				continue
			}

			result, err2 := stmt.Exec(nodedetail.NodeUUID, nodedetail.CPUModel, nodedetail.CPUProcessors, nodedetail.CPUThreads)
			if err2 != nil {
				logger.Logger.Println(err2)
				_ = stmt.Close()
				continue
			}
			_ = stmt.Close()

			logger.Logger.Println(result.LastInsertId())
		} else {
			sql = "update node_detail set cpu_model = ?, cpu_processors = ?, cpu_threads = ? where node_uuid = ?"
			stmt, err := mysql.Db.Prepare(sql)
			if err != nil {
				logger.Logger.Println(err)
				continue
			}

			result, err2 := stmt.Exec(nodedetail.CPUModel, nodedetail.CPUProcessors, nodedetail.CPUThreads, nodedetail.NodeUUID)
			if err2 != nil {
				logger.Logger.Println(err2)
				_ = stmt.Close()
				continue
			}
			_ = stmt.Close()

			if config.Ipmi.Debug == "on" {
				logger.Logger.Println(result.LastInsertId())
			}
		}

		nodedetails = append(nodedetails, nodedetail)
	}

	return nodedetails, nil
}

func queueCheckAll() {
	go func() {
		if config.Ipmi.Debug == "on" {
			logger.Logger.Println("queueCheckAll(): Rerun CheckAll() after " + strconv.Itoa(int(config.Ipmi.CheckAllIntervalMs)) + "ms")
		}
		delayMillisecond(time.Duration(config.Ipmi.CheckAllIntervalMs))
		CheckAll()
	}()
}
