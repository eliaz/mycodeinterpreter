package main
import (
	"fmt"
	"context"
	"os/exec"
	"time"
	"log"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"encoding/base64"
	"github.com/fatih/color"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"

)

var safeMode bool = true
var semisafe bool = false
var openAPISchema = ""

type OpenAPISchema struct {
	OpenAPI string                 `json:"openapi"`
	Info    map[string]interface{} `json:"info"`
	Paths   map[string]interface{} `json:"paths"`
}

// Colored logger for HTTP requests
var (
	infoLogger  = color.New(color.FgGreen).PrintfFunc()
	errorLogger = color.New(color.FgRed).PrintfFunc()
)



func getOpenAPISchema(ip string) string {
    return fmt.Sprintf(`openapi: 3.0.0
info:
  title: MyCodeInterpreter
  description: API for total control of a server
  version: 1.0.0
servers:
  - url: %s
    description: My Code Interpreter
paths:
  /get-file:
    get:
      operationId: get-file
      summary: Retrieves a file from the server.
      parameters:
        - name: filename
          in: query
          description: The name of the file to retrieve.
          required: true
          schema:
            type: string
      responses:
        '200':
          description: File content retrieved successfully.
        '400':
          description: 'Bad Request: Filename parameter missing.'
        '401':
          description: 'Unauthorized: Authentication credentials not provided or incorrect.'
        '404':
          description: 'Not Found: File not found on server.'
  /send-file:
    post:
      operationId: send-file
      summary: Uploads a file to the server.
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                document:
                  type: string
                  format: binary
                  description: The actual document
              required:
                - file
      responses:
        '200':
          description: File uploaded successfully.
        '400':
          description: 'Bad Request: File form field missing or no file provided.'
        '401':
          description: 'Unauthorized: Authentication credentials not provided or incorrect.'
        '500':
          description: 'Internal Server Error: Error occurred while processing the file.'
  /execcmd:
    post:
      operationId: execcmd
      summary: Execute a shell command on the server.
      parameters:
        - name: cmd
          in: query
          description: The name of the command to execute.
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Command executed successfully.
        '400':
          description: 'Bad Request: Command parameter missing.'
        '401':
          description: 'Unauthorized: Authentication credentials not provided or incorrect.'
        '403':
          description: 'Forbidden: Execution cancelled by admin or in safe mode.'
        '500':
          description: 'Internal Server Error: Error occurred while executing the command.'
`, ip)
}



func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		infoLogger("Request: %s %s\n", r.Method, r.URL.Path)
		handler.ServeHTTP(w, r)
	})
}



func CheckBasicAuth(authKey string, r *http.Request,w http.ResponseWriter) bool {
	if (authKey != "noauth"){ //we check basic auth
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return false
		}
		authEncoded := strings.TrimPrefix(authHeader, "Basic ")
		payload, _ := base64.StdEncoding.DecodeString(authEncoded)
		if (fmt.Sprintf("user:%s", authKey) != string(payload) )  {
			infoLogger("Basic auth failed, key was %s",payload)
			return false
		}
	}
        if (safeMode){ //SAFEMODE IS ON
       	       // Verification step
 	      fmt.Printf("Execute command? (y/n): ")
	       var response string
	       _, err := fmt.Scanln(&response)
	       if err != nil   {return false}
	       if response != "y" {
	                http.Error(w, "Execution cancelled by admin\n", http.StatusForbidden)
	                return false
	       }

       }else{
            infoLogger("WARNING SAFEMODE IS OFF, ABOUT TO EXECUTE!\n")
       }

       if (semisafe){
	    infoLogger("Semisafe is on, sleeping 2 seconds so that you can ctrl+c this madness before execution\n")
	    time.Sleep(2 * time.Second)
	    return true
	}
	return true

}

func getFileHandler(w http.ResponseWriter, r *http.Request, authKey string) {
	if !CheckBasicAuth(authKey, r,w) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	infoLogger("getFile permitted\n")

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Write(data)
}

func sendFileHandler(w http.ResponseWriter, r *http.Request, authKey string) {
	if !CheckBasicAuth(authKey, r,w) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	infoLogger("sendFile permited\n")

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	filename := "received_file" // Replace this with an appropriate file naming mechanism
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		http.Error(w, "Error writing file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File saved successfully as %s", filename)
}


func handleExecCmd(w http.ResponseWriter, r *http.Request, authKey string) {
	cmdStr := r.URL.Query().Get("cmd")
	if cmdStr == "" {
	       http.Error(w, "Bad Request: cmd parameter missing", http.StatusBadRequest)
	       return
        }
	infoLogger("Incomling cli exe:"+cmdStr+"\n")

	if !CheckBasicAuth(authKey, r,w) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
        }
   	infoLogger("Exec call permitted\n")	

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	infoLogger("Executing Cli request:"+cmdStr+"\n")
	  // Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) //for now always 60 sec, we could fork it after that and keep them running and return that its passed on as background process
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr) // Use CommandContext
	output, _ := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		w.WriteHeader(http.StatusRequestTimeout)
		w.Write([]byte("Command timed out"))
		return
	}
//	if err != nil{infoLogger("execution returned error:%e",err)} //TODO: we should probably do something here

	w.Header().Set("Content-Type", "text/plain")
	infoLogger("Execution response: %s\n", string(output))
	w.Write(output)
}


func setupRoutes(authKey string) {

	http.Handle("/", logRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})))
	http.Handle("/get-file", logRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		getFileHandler(w, r, authKey)
	})))
	http.Handle("/send-file", logRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendFileHandler(w, r, authKey)
	})))
	http.Handle("/execcmd", logRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                handleExecCmd(w, r, authKey)
        })))
	http.Handle("/openapi.json", logRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(openAPISchema))
	})))
}


func startNgrok(authKey string, ctx context.Context) error {
	infoLogger("starting ngrok")
	tun, err := ngrok.Listen(ctx,
		config.HTTPEndpoint(),
		ngrok.WithAuthtokenFromEnv(),
	)
	if err != nil {
		return err
	}

	log.Println("tunnel created:", tun.URL())
	address:=tun.URL()
        openAPISchema = getOpenAPISchema(address)
        infoLogger("OpenAPI schema at %s/openapi.json\n", address)
        infoLogger("Starting server at%s\n", address)
	setupRoutes(authKey)
	return  http.Serve(tun, nil)
}



func main() {
	if len(os.Args) < 2{
		log.Fatalln("Usage: NGROK_AUTHTOKEN=[your-key-here] ./mycodeinterpreter <authkey|noauth> [-nosafe] [-semisafe]")
	}
	for _, arg := range os.Args[1:] { // Skip the first argument which is the program name
        	if arg == "-nosafe" {
	            safeMode = false
        	}
	}
	for _, arg := range os.Args[1:] { 
                if arg == "-semisafe" {
                    semisafe = true
                }
        }

	authKey := os.Args[1]


        // Start ngrok  and http server
	if err := startNgrok(authKey,context.Background()); err != nil {
		log.Fatal(err)
	}



}





