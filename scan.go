package main

import (
	"os"
	"net"
	"log"
	"flag"
	"sync"
	"time"
	"strings"
	"strconv"
	"crypto/tls"
	"errors"
	"bufio"
)

type ScanConfig struct{
	Threads int
	LogFile string
	PathFile string
	Method string
	DomainFile string
	CollaboratorClient string
}

type JobData struct{
	TlsConf *tls.Config
	Domain string
	Path   string
	Method string
	CollaboratorClient string
	Port string
	UseSSL bool
}

type DomainInfo struct{
	Domain string
	Ports  []string
}

func (c *ScanConfig) ParseCommandLineFlags() {
	flag.StringVar(&c.LogFile, "log", "scaner.log", "Log file")
	flag.StringVar(&c.PathFile, "p", "pathes.txt", "Files with pathes to visit")
	flag.StringVar(&c.DomainFile, "d", "domains.txt", "File with domais")
	flag.StringVar(&c.CollaboratorClient, "c", "", "Collaborator client")
	flag.StringVar(&c.Method, "m", "GET", "HTTP method")
	flag.IntVar(&c.Threads, "j", 5,  "Number of gorutines")
	flag.Parse()
}

func doSSLSocketRequest(conf *tls.Config, dialer *net.Dialer, address string, request string){
	conn, err := tls.DialWithDialer(dialer, "tcp", address, conf)
    if err != nil {
        log.Println(err)
        return
	}

	_, err = conn.Write([]byte(request))

	if err != nil {
        log.Println(err)
        return
	}

	<- time.After(10 * time.Millisecond)
	conn.Close() // Don't wait for response
}

func doSocketRequest(dialer *net.Dialer, address string, request string){
	conn, err := dialer.Dial( "tcp", address)
    if err != nil {
        log.Println(err)
        return
	}

	_, err = conn.Write([]byte(request))

	if err != nil {
        log.Println(err)
        return
	}

	<- time.After(10 * time.Millisecond)
	conn.Close() // Don't wait for response
}

func readFile(filename string)([]string, error){
	file, err := os.Open(filename)
	if err != nil{
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
	}
	
    return lines, scanner.Err()
}

func prepareRequest(domain string, path string, method string, collaboratorClient string)(string){
	var webRequestBody strings.Builder
	webRequestBody.WriteString(method + " " + path + "?domain=" + domain + " HTTP/1.1\r\n")
	webRequestBody.WriteString("Host: " + collaboratorClient + "\r\n")
	webRequestBody.WriteString("User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:81.0) Gecko/20100101 Firefox/81.0\r\n")
	webRequestBody.WriteString("Accept: text/html;q=0.9,*/*;q=0.8\r\n")
	webRequestBody.WriteString("Connection: Close\r\n\r\n")
	return webRequestBody.String()
}

func readDomainInfo(file string)(domainsInfo []DomainInfo, err error){
	domainsInfoData, err := readFile(file)

	if err != nil {
		return nil, err
	}

	for _, domainInfo := range domainsInfoData{ 
		tmp := strings.SplitN(domainInfo, " ", -1)
		if len(tmp) < 2 {
			err = errors.New("Wrong format of the " + file)
		}

		var domainInfo DomainInfo
		domainInfo.Domain = tmp[0]

		for _, portData := range(strings.SplitN(tmp[1], ",", -1)){	
			_, err := strconv.Atoi(portData) // Verify integer type
			if err != nil{
				return nil, errors.New("Wrong format of the "+file)
			}

			domainInfo.Ports = append(domainInfo.Ports, portData)
		}

		domainsInfo = append(domainsInfo, domainInfo)
	}

	return domainsInfo, err
}

func doWork(jobChan chan JobData, wg *sync.WaitGroup){
	defer wg.Done()
	dialer := net.Dialer{Timeout: 5 * time.Second}

	for jobData := range(jobChan){
		address := jobData.Domain + ":" + jobData.Port
		request := prepareRequest(jobData.Domain, jobData.Path, jobData.Method, jobData.CollaboratorClient)
		
		if jobData.UseSSL{
			doSSLSocketRequest(jobData.TlsConf, &dialer, address, request)
		} else{
			doSocketRequest(&dialer, address, request)
		}
	}

	return
}

func main(){
	config := &ScanConfig{}
	config.ParseCommandLineFlags()
	jobChannelCapacity := 8192

	logf, err := os.OpenFile(config.LogFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	
	if err != nil {
		log.Fatalf("error opening file: %v", err)
		return
	}
	defer logf.Close()

	log.SetOutput(logf)
	log.SetFlags(log.Ltime|log.Lshortfile)

	domainsInfo, err := readDomainInfo(config.DomainFile)
	if err != nil{
		log.Fatalf("%v", err)
		return
	}

	pathes, err := readFile(config.PathFile)
	if err != nil{
		log.Fatalf("error opening file: %v", err)
		return
	}

	var wg sync.WaitGroup
	jobChan := make(chan JobData, jobChannelCapacity)
	
	tlsConf := tls.Config{
		InsecureSkipVerify: true,
	}

	for i := 0; i < config.Threads; i++ {
		wg.Add(1)
		go doWork(jobChan, &wg)
	}

	log.Println("Scan has been started...")
	defer log.Println("Scan finished succesesfully!")

	for _, domainInfo := range(domainsInfo) {
		for _, port := range(domainInfo.Ports){
			for _, path := range(pathes){
				var newJob JobData
				newJob.Port = port
				newJob.Domain = domainInfo.Domain
				newJob.Path = path
				newJob.Method = config.Method
				newJob.CollaboratorClient = config.CollaboratorClient
				newJob.TlsConf = &tlsConf

				if port == "443"{
					newJob.UseSSL = true
				} else {
					newJob.UseSSL = false
				}

				jobChan <- newJob				
			}
		}
	}
	
	close(jobChan)
	wg.Wait()
}