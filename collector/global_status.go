package collector

import (
	"context"
	"database/sql"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
)

const (
	//Exporter Namespace
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
	globalHandlerDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "handlers_total"),
		"Total number of executed MySQL handlers.",
		[]string{"handler"}, nil,
	)
)

type ScrapeGlobalStatus struct{}

func (ScrapeGlobalStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {

	//1.连接数据库并且执行查询
	globalStatusRows, err := db.QueryContext(ctx, globalStatusQuery)
	if err != nil {
		return err
	}
	defer db.Close()
	//2.遍历匹配行的值
	var key string
	var val sql.RawBytes
	var textItems = map[string]string{
		"wsrep_local_state_uuid":   "",
		"wsrep_cluster_state_uuid": "",
		"wsrep_provider_version":   "",
		"wsrep_evs_repl_latency":   "",
	}

	for globalStatusRows.Next() {
		if err := globalStatusRows.Scan(&key, &val); err != nil {
			return err
		}
		key = validPrometheusName(key)
		if floatVal, ok := parseStatus(val); ok {
			key = validPrometheusName(key)
			match := globalStatusRE.FindStringSubmatch(key)
			if match == nil {
				ch <- prometheus.MustNewConstMetric(
					newDesc(globalStatus, key, "Generic metric from SHOW GLOBAL STATUS."),
					prometheus.UntypedValue,
					floatVal,
				)
				continue
			}
			switch match[1] {
			case "com":
				ch <- prometheus.MustNewConstMetric(
					globalCommandsDesc, prometheus.CounterValue, floatVal, match[2],
				)

			case "handler":
				ch <- prometheus.MustNewConstMetric(
					globalHandlerDesc, prometheus.CounterValue, floatVal, match[2],
				)

			}

		} else if _, ok := textItems[key]; ok {
			textItems[key] = string(val)
		}
		return nil
	}

	return nil
}

// Regexp to match various groups of status vars.
var globalStatusRE = regexp.MustCompile(`^(com|handler|connection_errors|innodb_buffer_pool_pages|innodb_rows|performance_schema)_(.*)$`)

func (ScrapeGlobalStatus) Name() string {
	return globalStatus
}
func (ScrapeGlobalStatus) Version() float64 {
	return 5.7
}
func (ScrapeGlobalStatus) Help() string {
	return "Collect from SHOW GLOBAL STATUS"
}

var _ Scraper = ScrapeGlobalStatus{}
