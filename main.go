package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Job struct {
	Name   string
	Delay  time.Duration
	Number int
}

type Worker struct {
	Id       int
	JobQueue chan Job
	//canal de canales
	WorkerPool chan chan Job
	QuitChan   chan bool
}

type Dispatcher struct {
	WorkerPool chan chan Job
	MaxWorkers int
	//Cola de trabajo
	JobQueue chan Job
}

// Constrcutor en forma de funcion del worker se recibe el id, el canal de canales de trabajo y se returna el valor del worker
// Ademas se crea el canal de la cola de trabajo y el canal de worker que se saldran
func NewWork(id int, workerPool chan chan Job) *Worker {
	return &Worker{
		Id:         id,
		JobQueue:   make(chan Job),
		WorkerPool: workerPool,
		QuitChan:   make(chan bool),
	}
}

func (w Worker) Start() {
	go func() {
		for {
			w.WorkerPool <- w.JobQueue
			//Si lo que viene es de la cola de trabajo para hacer hara el calculo de fibonacci y terminara su labor, si viene la cola de quit se cerrara el worker
			select {
			case job := <-w.JobQueue:
				fmt.Printf("Worker with id %d Started\n", w.Id)
				fib := Fibonacci(job.Number)
				time.Sleep(job.Delay)
				fmt.Printf("Worker with id %d Finished with result %d \n", w.Id, fib)
			case <-w.QuitChan:
				fmt.Printf("Worker with id %d Stopped\n", w.Id)
			}
		}
	}()
}

func (w Worker) Stop() {
	go func() {
		w.QuitChan <- true
	}()
}

func Fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return Fibonacci(n-1) + Fibonacci(n-2)
}

// Constructor del dispatcher
func NewDispatcher(jobQueue chan Job, maxWokers int) *Dispatcher {
	//Creamos el canal que se va a encargar de ver cuantos trabajadores hay listos para trabajar y enviandoles trabajo, lo estamos trabajando con un buffer para poder limitar la cantidad maxima de workers que haya y nos aseguramos de no tener concurrencia ilimitada
	worker := make(chan chan Job, maxWokers)
	return &Dispatcher{
		JobQueue:   jobQueue,
		MaxWorkers: maxWokers,
		//Sabremos cuando le estamos enviado jobs a nuestros wokers
		WorkerPool: worker,
	}
}

func (d *Dispatcher) Dispatch() {
	for {
		select {
		case job := <-d.JobQueue:
			go func() {
				//acá recibiremos el pool de trabajadores que necesitaremos para encolar nuestro trabajo
				workerJobQueue := <-d.WorkerPool
				workerJobQueue <- job
			}()
		}
	}
}

func (d *Dispatcher) Run() {
	//Crearemos workers en la medida de la iteracion con el i del indicie del for y tendramos maximo la cantidad de workers creada en el disaptch
	for i := 0; i < d.MaxWorkers; i++ {
		//Usaremos para crear al worker, el workerpool que esta dentro del dispatcher
		worker := NewWork(i, d.WorkerPool)
		worker.Start()
	}

	//llamaremos en formato de go routine al dispatch para que empiece a despachar tareas
	go d.Dispatch()
}

// Funcion encargada de manejar todo lo que el server deba ser capaz de procesar
// ResponseWriter nos ayudara a escribir las respuestas para los que soliciten al server
// r *http.Request Nos permitira ver la request que estamos recibiendo
func RequestHandler(w http.ResponseWriter, r *http.Request, jobQueue chan Job) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

	//FormValue -> nos permitira acceder a todo lo que se esta enviando en el request
	delay, err := time.ParseDuration(r.FormValue("delay"))
	if err != nil {
		http.Error(w, "Invalid Delay", http.StatusBadRequest)
		return
	}

	//Atoi me permite pasarle un string y convertirlo a entero
	value, err := strconv.Atoi(r.FormValue("value"))
	if err != nil {
		http.Error(w, "Invalid Value", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")

	if name == "" {
		http.Error(w, "Invalid Name", http.StatusBadRequest)
		return
	}

	//Creamos el job
	job := Job{Name: name, Delay: delay, Number: value}
	//Encolamos nuestro trabajo
	jobQueue <- job
	//Le avisamos al cliente que su peticion fue correcta y esta en trabajo
	w.WriteHeader(http.StatusCreated)
}

func main() {
	const (
		//cantidad maxima de workers
		maxWokers = 4
		//cantidad maxima de tareas que podran ser atendidad
		maxQueueSize = 20
		//puerto que escuchara la aplicacion
		port = ":8081"
	)
	//Encargado de manejar todos los jobs que se reciban en las peticiones en formato buffered channel
	jobQueue := make(chan Job, maxQueueSize)
	//Se le pasa por parametro el chanel que recebiria los jobs y la cantidad maxima de workers que tendra
	dispatcher := NewDispatcher(jobQueue, maxWokers)

	dispatcher.Run()

	//http://localhost:8081/fib
	//Con handle func estaremos estableciendo lo referente a como si fuera un controlador
	http.HandleFunc("/fib", func(w http.ResponseWriter, r *http.Request) {
		RequestHandler(w, r, jobQueue)
	})
	// Log correra si nuestra aplicacion deja de funcionar podemos ver que paso
	// con http.ListenAndServe(port, nil) levantamos nuestro server en un puerto
	// Aparte de esta manera bloqueara el programa hasta que o rompa por un error o yo lo detenga
	log.Fatal(http.ListenAndServe(port, nil))
}
