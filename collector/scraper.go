package collector

import (
	"context"
	"database/sql"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)


type Scraper interface {
	// Name of the Scraper. Should be unique.
	Name()string

	// Help describes the role of the Scraper.
	Help()string

	//Version of Mysql
	Version()float64

	// Scrape collects data from database connection and sends it over channel as prometheus metric.
	Scrape(ctx context.Context,db *sql.DB,ch chan<-prometheus.Metric,logger log.Logger )error
}