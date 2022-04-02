package collector

import (
	"context"
	"database/sql"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	//Exporter Namespace
	namespace = "mysql"
	// Scrape query.
	globalStatusQuery = `SHOW GLOBAL STATUS`
	// Subsystem.
	globalStatus = "global_status"
)


// Metric descriptors.
var (
	globalCommandsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "commands_total"),
		"Total number of executed MySQL commands.",
		[]string{"command"}, nil,
	)
)



type ScrapeGlobalStatus struct{}

func (ScrapeGlobalStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {

	//1.连接数据库并且执行查询
	//2.



	return nil
}

func(ScrapeGlobalStatus)Name()string{
	return globalStatus
}
func (ScrapeGlobalStatus)Version()float64 {
	return 5.7
}
func (ScrapeGlobalStatus)Help()string{
	return "Collect from SHOW GLOBAL STATUS"
}

var _ Scraper = ScrapeGlobalStatus{}