/*@Author: LinkLeong link@icewhale.com
 *@Date: 2021-12-08 18:10:25
 *@LastEditors: LinkLeong
 *@LastEditTime: 2022-07-13 10:49:16
 *@FilePath: /CasaOS/model/docker.go
 *@Description:
 *@Website: https://www.casaos.io
 *Copyright (c) 2022 by icewhale, All Rights Reserved.
 */
package model

type DockerStatsModel struct {
	Icon     string      `json:"icon"`
	Title    string      `json:"title"`
	Data     interface{} `json:"data"`
	Previous interface{} `json:"previous"`

	// Computed from Data/Previous's "networks" field (not part of Docker's
	// own stats payload) since the frontend needs a ready-to-use rate, not
	// two raw cumulative byte counters to diff itself.
	NetworkRxBytesPerSec float64 `json:"network_rx_bytes_per_sec"`
	NetworkTxBytesPerSec float64 `json:"network_tx_bytes_per_sec"`
}

// reference - https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file
type DockerDaemonConfigurationModel struct {
	// e.g. `/var/lib/docker`
	Root string `json:"data-root,omitempty"`
}
