package main

import (
	"context"
	"fmt"
	"github.com/go-kit/log"
	_ "github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/ini.v1"
	_ "log"
	"mysql_exporter_demo/collector"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	prometheus.MustRegister(version.NewCollector("mysqld_exporter"))
}
//1.定义启动参数
var (
	webConfig     = pflag.CommandLine
	listenAddress = pflag.String("web.listen-address","127.0.0.1:9999","Address to listen on for web interface and telemetry.")
	metricPath = pflag.String("web.telemetry-path","/metrics","Path under which to expose metrics.")
	timeoutOffset = pflag.Float64("timeout-offset",0.25,"Offset to subtract from timeout in seconds.")
	configMycnf = pflag.String("config.my-cnf","D:\\my.cnf", "Path to .my.cnf file to read MySQL credentials from.")
	dsn string
)



// 2.数据抓取的func
var scrapers = map[collector.Scraper]bool{
	collector.ScrapeGlobalStatus{}:true,
}

func main()  {
	// Generate ON/OFF flags for all scrapers.
	scraperFlags := map[collector.Scraper]*bool{}
	for scrape,enabledByDefault := range scrapers {
		defaultOn := "false"
		if  enabledByDefault {
			defaultOn = "true"
		}

		//校验参数
		strdefaultOn,_ := strconv.ParseBool(defaultOn)
		f := pflag.Bool("collect."+scrape.Name(),strdefaultOn,scrape.Help())
		scraperFlags[scrape] = f
		fmt.Println(*f,scrape)
	}

	// Parse flags.
	promlogConfig := &promlog.Config{}
	flag.AddFlags( kingpin.CommandLine,promlogConfig)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger :=promlog.New(promlogConfig)

	// landingPage contains the HTML served at '/'.

	// landingPage contains the HTML served at '/'.
	// TODO: Make this nicer and more informative.
	var landingPage = []byte(`<html>
<head><title>MySQLd exporter</title></head>
<body>
<h1>MySQLd exporter</h1>
<p><a href='` + *metricPath + `'>Metrics</a></p>
</body>
</html>
`)

	logrus.Infof("Starting mysqld_exporter,version is %v",version.Info())
	logrus.Infof("Build context,%v",version.BuildContext())


	if len(dsn) == 0 {
		var err error
		if dsn, err = parseMycnf(*configMycnf); err != nil {
			logrus.Errorf("Error parsing my.cnf:%v",err)
			os.Exit(1)
		}
	}

	// Register only scrapers enabled by flag.
	enabledScrapers :=[]collector.Scraper{}
	for scraper,enabled := range scraperFlags{
		if *enabled{
			logrus.Infof("Scraper enabled:%v",scraper.Name())
			enabledScrapers = append(enabledScrapers,scraper)
		}

	}

	handlerFunc :=newHandler(collector.NewMetrics(),enabledScrapers,logger)
	http.Handle(*metricPath,promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer,handlerFunc))
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write(landingPage)
	})
	logrus.Infof("Listening on address:%v",*listenAddress)
	srv:=&http.Server{
		Addr: *listenAddress,
	}
	if err:= web.ListenAndServe(srv,"",logger);err != nil{
		logrus.Errorf("Error starting HTTP server:%v",err)
		os.Exit(1)
	}

}

func newHandler(metrics collector.Metrics, Scrapers []collector.Scraper, logger log.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter,r *http.Request) {
		//logger.Println("")
		logrus.Info("newhandler func running")
		//w.Write([]byte("newhandler"+*configMycnf))

		filteredScrapers :=Scrapers
		params := r.URL.Query()["collect[]"]
		ctx :=r.Context()
		if v:=r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds");v !=""{
			timeoutSeconds ,err :=strconv.ParseFloat(v,64)
			if err != nil{
				logrus.Infof("Failed to parse timeout from Prometheus header:%v",err)
			}else {
				if *timeoutOffset >= timeoutSeconds{
					// Ignore timeout offset if it doesn't leave time to scrape.
					logrus.Infof("Timeout offset should be lower than prometheus scrape timeout,offset:%v,timeout:%v",*timeoutOffset,timeoutSeconds)

				}else {
					// Subtract timeout offset from timeout.
					timeoutSeconds -= *timeoutOffset
				}
				// Create new timeout context with request context as parent.
				var cancel context.CancelFunc
				ctx,cancel= context.WithTimeout(ctx,time.Duration(timeoutSeconds *float64(time.Second)))
				defer cancel()
				// Overwrite request with timeout context.
				r = r.WithContext(ctx)
			}

		}

		logrus.Debugf("collect[] params:%v",strings.Join(params, ","))
		// Check if we have some "collect[]" query parameters.
		if len(params)>0{
			filters :=make(map [string]bool)
			for _,param :=range params{
				filters[param] =true
			}
			filteredScrapers = nil
			for _,scraper := range Scrapers{
				if filters[scraper.Name()]{
					filteredScrapers = append(filteredScrapers,scraper)
				}
			}
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector.New(ctx,dsn,metrics,filteredScrapers,logger))
		//fmt.Println(dsn)
		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}
		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h :=promhttp.HandlerFor(gatherers,promhttp.HandlerOpts{})
		h.ServeHTTP(w,r)
	}
}

func parseMycnf(config interface{}) (string, error) {
	logrus.Info("parseMycnf func running")
	var dsn string
	opts := ini.LoadOptions{
		// MySQL ini file can have boolean keys.
		AllowBooleanKeys: true,
	}
	cfg,err := ini.LoadSources(opts,config)
	if err != nil{
		return dsn,fmt.Errorf("failed reading ini file: %s", err)
	}
	user := cfg.Section("client").Key("user").String()
	password := cfg.Section("client").Key("password").String()
	if user == "" {
		return dsn, fmt.Errorf("no user specified under [client] in %s", config)
	}
	host := cfg.Section("client").Key("host").MustString("localhost")
	port := cfg.Section("client").Key("port").MustUint(3306)
	socket := cfg.Section("client").Key("socket").String()
	sslKey := cfg.Section("client").Key("ssl-key").String()
	passwordPart := ""
	if password != "" {
		passwordPart = ":" + password
	} else {
		if sslKey == "" {
			return dsn, fmt.Errorf("password or ssl-key should be specified under [client] in %s", config)
		}
	}
	if socket != "" {
		dsn = fmt.Sprintf("%s%s@unix(%s)/", user, passwordPart, socket)
	} else {
		dsn = fmt.Sprintf("%s%s@tcp(%s:%d)/", user, passwordPart, host, port)
	}

	//fmt.Println("return dsn"+dsn)
	return dsn,nil
}
