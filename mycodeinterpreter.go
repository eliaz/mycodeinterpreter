package main
import (
	"fmt"
	"context"
	"os/exec"
	"time"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"encoding/base64"
	"log"
	"github.com/fatih/color"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
 	 ngrok_log "golang.ngrok.com/ngrok/log"

)

var safeMode bool = true
var semisafe bool = false
var openAPISchema = ""

// Colored logger for HTTP requests
var (
    logger *log.Logger
)

func init() {
    // Create a custom logger
    logger = log.New(os.Stdout, "", 0)
}

func mlog(logType, format string, a ...interface{}) {
    var message string
    switch logType {
    case "info":
        message = color.GreenString(fmt.Sprintf(format, a...))
    case "error":
        message = color.RedString(fmt.Sprintf(format, a...))
    default:
        message = fmt.Sprintf(format, a...) // No color for undefined types
    }
    logger.Println(message)
}


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
		mlog("info","Request: %s %s", r.Method, r.URL.Path)
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
			mlog("info","Basic auth failed, key was %s",payload)
			return false
		}
	}
        if (safeMode){ //SAFEMODE IS ON
       	       // Verification step
 	      mlog("error","Execute command? (y/n): ")
	       var response string
	       _, err := fmt.Scanln(&response)
	       if err != nil   {return false}
	       if response != "y" {
	                http.Error(w, "Execution cancelled by admin", http.StatusForbidden)
			mlog("error","Aborting execution")
	                return false
	       }

       }else{
            mlog("info","WARNING SAFEMODE IS OFF, ABOUT TO EXECUTE!\n")
       }

       if (semisafe){
	    mlog("info","Semisafe is on, sleeping 2 seconds so that you can ctrl+c this madness before execution\n")
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
	mlog("info","getFile permitted\n")

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}
	mlog("info","Fetching file %s",filename)

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Write(data)
}

func sendFileHandler(w http.ResponseWriter, r *http.Request, authKey string) {
 // Check for basic authentication
    if !CheckBasicAuth(authKey, r, w) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
	mlog("info","unauthorized")
        return
    }
    mlog("info", "sendFile permitted")

    // Only accept POST requests
    if r.Method != "POST" {
        http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	mlog("error","Invalid request method")
        return
    }

    // Parse the multipart form (max upload size set to 10 MB)
    err := r.ParseMultipartForm(10 << 20)
    if err != nil {
        http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
	mlog("error","Error parsing multipart form")
        return
    }

    // Retrieve the file from form data
    file, handler, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "Error retrieving the file", http.StatusBadRequest)
	mlog("error","Error parsing multipart form")
        return
    }
    defer file.Close()

    // Read the file content
    fileData, err := ioutil.ReadAll(file)
    if err != nil {
        http.Error(w, "Error reading the file", http.StatusInternalServerError)
	mlog("error","Error reading the file")
        return
    }

    // Write the file to the server with the original filename
    err = ioutil.WriteFile(handler.Filename, fileData, 0644)
    if err != nil {
        http.Error(w, "Error writing the file", http.StatusInternalServerError)
	mlog("error","Error writing the file")
        return
    }
    mlog("info","File saved successfully as %s",handler.Filename)
    fmt.Fprintf(w, "File saved successfully as %s", handler.Filename)
}


func handleExecCmd(w http.ResponseWriter, r *http.Request, authKey string) {
	cmdStr := r.URL.Query().Get("cmd")
	if cmdStr == "" {
	       http.Error(w, "Bad Request: cmd parameter missing", http.StatusBadRequest)
	       return
        }
	mlog("info","Incomling cli exe:"+cmdStr)

	if !CheckBasicAuth(authKey, r,w) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
		mlog("error","returning unauthorized due to basicauth fail")
                return
        }
   	mlog("info","Exec call permitted")

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		mlog("error","Incorrect request (post)")

		return
	}

	mlog("info","Executing Cli request:"+cmdStr+"")
	  // Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) //for now always 300 sec, we could fork it after that and keep them running and return that its passed on as background process
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
	mlog("info","Execution response: %s\n", string(output))
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



// Simple logger that forwards to the Go standard logger.
type grocklogger struct {
	lvl ngrok_log.LogLevel
}

func (l *grocklogger) Log(ctx context.Context, lvl ngrok_log.LogLevel, msg string, data map[string]interface{}) {
	if lvl > l.lvl {
		return
	}
	lvlName, _ := ngrok_log.StringFromLogLevel(lvl)
	log.Printf("[%s] %s %v", lvlName, msg, data)
}

func startNgrok(authKey string, ctx context.Context) error {
	mlog("info","Starting ngrok")
	lvl, err := ngrok_log.LogLevelFromString("info")
	if err != nil {
		log.Printf("%s",err)
	}
	ngrokAuthToken := os.Getenv("NGROK_AUTHTOKEN")

    	if ngrokAuthToken == "" {
        	log.Fatal("NGROK_AUTHTOKEN is not set")
        }
	tun, err := ngrok.Listen(ctx,
		config.HTTPEndpoint(),
		ngrok.WithAuthtoken(ngrokAuthToken),
		ngrok.WithLogger(&grocklogger{lvl}),
	)
	if err != nil {
		return err
	}

	log.Println("tunnel created:", tun.URL())
	address:=tun.URL()
        openAPISchema = getOpenAPISchema(address)
        mlog("info","OpenAPI schema at %s/openapi.json", address)
        mlog("info","Starting server at %s", address)
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





