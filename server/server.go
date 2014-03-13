package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/simonleung8/pat/benchmarker"
	"github.com/simonleung8/pat/config"
	. "github.com/simonleung8/pat/experiment"
	"github.com/simonleung8/pat/experiments"
	. "github.com/simonleung8/pat/laboratory"
	"github.com/simonleung8/pat/store"
)

const (
	PortVar = "VCAP_APP_PORT"
)

type Response struct {
	TotalTime int64
	Timestamp int64
}

type context struct {
	router *mux.Router
	lab    Laboratory
}

var params = struct {
	csvDir string
}{}

func InitCommandLineFlags(flags config.Config) {
	flags.StringVar(&params.csvDir, "csvDir", "output/csvs", "Directory to Store CSVs")
}

func Serve() {
	ServeWithArgs(params.csvDir)
}

func ServeWithArgs(csvDir string) {
	ServeWithLab(NewLaboratory(store.NewCsvStore(csvDir)))
}

func ServeWithLab(lab Laboratory) {
	r := mux.NewRouter()
	ctx := &context{r, lab}

	r.Methods("GET").Path("/experiments/").HandlerFunc(handler(ctx.handleListExperiments))
	r.Methods("GET").Path("/experiments/{name}.csv").HandlerFunc(csvHandler(ctx.handleGetExperiment)).Name("csv")
	r.Methods("GET").Path("/experiments/{name}").HandlerFunc(handler(ctx.handleGetExperiment)).Name("experiment")
	r.Methods("POST").Path("/experiments/").HandlerFunc(handler(ctx.handlePush))
	r.Methods("GET").Path("/").HandlerFunc(redirectBase)
	http.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("ui"))))
	http.Handle("/", r)
}

func redirectBase(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ui", http.StatusFound)
}

func Bind() {
	var port string
	if port = os.Getenv(PortVar); port == "" {
		port = "8080"
	}

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}

type listResponse struct {
	Items interface{}
}

func (ctx *context) handleListExperiments(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	experiments := make([]map[string]string, 0)
	ctx.lab.Visit(func(e Experiment) {
		json := make(map[string]string)
		url, _ := ctx.router.Get("experiment").URL("name", e.GetGuid())
		csvUrl, _ := ctx.router.Get("csv").URL("name", e.GetGuid())
		json["Location"] = url.String()
		json["CsvLocation"] = csvUrl.String()
		json["Name"] = "Simple Push (" + e.GetGuid() + ")"
		json["State"] = "Unknown"
		experiments = append(experiments, json)
	})

	return &listResponse{experiments}, nil
}

func (ctx *context) handlePush(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	pushes, err := strconv.Atoi(r.FormValue("iterations"))
	if err != nil {
		pushes = 1
	}

	concurrency, err := strconv.Atoi(r.FormValue("concurrency"))
	if err != nil {
		concurrency = 1
	}

	interval, err := strconv.Atoi(r.FormValue("interval"))
	if err != nil {
		interval = 0
	}
	stop, err := strconv.Atoi(r.FormValue("stop"))
	if err != nil {
		stop = 0
	}

	workload := r.FormValue("workload")
	if workload == "" {
		workload = "push"
	}

	//ToDo (simon): interval and stop is 0, repeating at interval is not yet exposed in Web UI
	worker := benchmarker.NewWorker()
	worker.AddExperiment("login", experiments.Dummy)
	worker.AddExperiment("push", experiments.Push)
	worker.AddExperiment("dummy", experiments.Dummy)
	worker.AddExperiment("dummywitherrors", experiments.DummyWithErrors)
	experiment, _ := ctx.lab.Run(
		NewRunnableExperiment(
			NewExperimentConfiguration(
				pushes, concurrency, interval, stop, worker, workload)))

	return ctx.router.Get("experiment").URL("name", experiment.GetGuid())
}

func (ctx *context) handleGetExperiment(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	name := mux.Vars(r)["name"]
	// TODO(jz) only send back since N
	data, err := ctx.lab.GetData(name)
	return &listResponse{data}, err
}

func csvHandler(fn func(http.ResponseWriter, *http.Request) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if response, err := fn(w, r); err == nil {
			fmt.Fprintf(w, "Average,TotalTime,Total,TotalErrors,TotalWorkers,LastResult,LastError,WorstResult,WallTime,Type\n")
			for _, line := range response.(*listResponse).Items.([]*Sample) {
				fmt.Fprintf(w, "%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
					line.Average, line.TotalTime, line.Total, line.TotalErrors, line.TotalWorkers, line.LastResult, line.LastError, line.WorstResult, line.WallTime, line.Type)
			}
		}
	}
}

func handler(fn func(http.ResponseWriter, *http.Request) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response interface{}
		var encoded []byte

		if response, err = fn(w, r); err == nil {
			switch r := response.(type) {
			case *url.URL:
				w.Header().Set("Location", r.String())
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, "{ \"Location\": \"%v\", \"CsvLocation\": \"/csv/%v.csv\" }", r, r)
				return
			default:
				if encoded, err = json.Marshal(r); err == nil {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, string(encoded))
					return
				}
			}
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
