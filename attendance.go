package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func encryptPassword(plaintext, key, iv string) (string, error) {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	paddedText := append([]byte(plaintext), bytes.Repeat([]byte{byte(padding)}, padding)...)

	mode := cipher.NewCBCEncrypter(block, ivBytes)

	cipherText := make([]byte, len(paddedText))
	mode.CryptBlocks(cipherText, paddedText)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func FetchAttendance(studentID, password string) (string, error) {
	loginURL := "https://webprosindia.com/vignanit/default.aspx"
	attendanceURL := "https://webprosindia.com/vignanit/Academics/studentacadamicregister.aspx"

	client := &http.Client{}

	loginPageResp, err := client.Get(loginURL)
	if err != nil {
		return "", err
	}
	defer loginPageResp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(loginPageResp.Body)
	if err != nil {
		return "", err
	}

	viewState := doc.Find("input[name='__VIEWSTATE']").AttrOr("value", "")
	viewStateGenerator := doc.Find("input[name='__VIEWSTATEGENERATOR']").AttrOr("value", "")
	eventValidation := doc.Find("input[name='__EVENTVALIDATION']").AttrOr("value", "")

	key := "8701661282118308"
	iv := "8701661282118308"
	encryptedPassword, err := encryptPassword(password, key, iv)
	if err != nil {
		return "", err
	}

	data := url.Values{}
	data.Set("__VIEWSTATE", viewState)
	data.Set("__VIEWSTATEGENERATOR", viewStateGenerator)
	data.Set("__EVENTVALIDATION", eventValidation)
	data.Set("txtId2", studentID)
	data.Set("txtPwd2", password)
	data.Set("imgBtn2.x", "0")
	data.Set("imgBtn2.y", "0")
	data.Set("hdnpwd2", encryptedPassword)

	loginReq, err := http.NewRequest("POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	loginReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Add("Origin", "https://webprosindia.com")
	loginReq.Header.Add("Referer", loginURL)
	loginReq.Header.Add("User-Agent", "Mozilla/5.0")

	loginResp, err := client.Do(loginReq)
	if err != nil {
		return "", err
	}
	defer loginResp.Body.Close()

	cookies := loginResp.Cookies()
	var frmAuth, sessionID string
	for _, cookie := range cookies {
		if cookie.Name == "frmAuth" {
			frmAuth = cookie.Value
		}
		if cookie.Name == "ASP.NET_SessionId" {
			sessionID = cookie.Value
		}
	}

	if frmAuth == "" || sessionID == "" {
		return "", fmt.Errorf("failed to retrieve login cookies")
	}

	attendanceReq, err := http.NewRequest("GET", attendanceURL, nil)
	if err != nil {
		return "", err
	}
	q := attendanceReq.URL.Query()
	q.Add("scrid", "2")
	attendanceReq.URL.RawQuery = q.Encode()

	attendanceReq.Header.Add("Cookie", fmt.Sprintf("ASP.NET_SessionId=%s; frmAuth=%s", sessionID, frmAuth))
	attendanceReq.Header.Add("Referer", "https://webprosindia.com/vignanit/StudentMaster.aspx")
	attendanceReq.Header.Add("User-Agent", "Mozilla/5.0")

	attendanceResp, err := client.Do(attendanceReq)
	if err != nil {
		return "", err
	}
	defer attendanceResp.Body.Close()

	attendanceDoc, err := goquery.NewDocumentFromReader(attendanceResp.Body)
	if err != nil {
		return "", err
	}

	var dataRows [][]string
	attendanceTable := attendanceDoc.Find("#tblReport table")
	attendanceTable.Find("tr").Each(func(i int, row *goquery.Selection) {
		var rowData []string
		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			rowData = append(rowData, strings.TrimSpace(cell.Text()))
		})
		dataRows = append(dataRows, rowData)
	})

	if len(dataRows) < 4 {
		return "", fmt.Errorf("failed to parse attendance data")
	}

	rollNumber := strings.Replace(dataRows[3][1], "\u00a0", "", -1)

	var cleanedData [][]string
	for _, row := range dataRows[7:] {
		var cleanedRow []string
		for _, cell := range row {
			cleanedRow = append(cleanedRow, strings.Replace(cell, "\xa0", "", -1))
		}
		cleanedData = append(cleanedData, cleanedRow)
	}

	attendanceSummary := []map[string]interface{}{}
	subjectwiseSummary := []map[string]interface{}{}
	totalAttended, totalHeld := 0, 0
	for _, row := range cleanedData[1:] {
		attendedHeld := row[len(row)-2]
		attendedHeldSplit := strings.Split(attendedHeld, "/")
		attended := 0
		held := 0
		fmt.Sscanf(attendedHeldSplit[0], "%d", &attended)
		fmt.Sscanf(attendedHeldSplit[1], "%d", &held)
		totalAttended += attended
		totalHeld += held

		if attendedHeld != "0/0" {
			subjectwiseSummary = append(subjectwiseSummary, map[string]interface{}{
				"subject_name": row[1],
				"attended_held": attendedHeld,
				"percentage": row[len(row)-1],
			})
		}
	}

	totalPercentage := float64(totalAttended) / float64(totalHeld) * 100
	totalInfo := map[string]interface{}{
		"total_attended": totalAttended,
		"total_held":     totalHeld,
		"total_percentage": totalPercentage,
	}

	if totalPercentage < 75 {
		additionalHours := (0.75 * float64(totalHeld) - float64(totalAttended)) / (1 - 0.75)
		totalInfo["additional_hours_needed"] = int(additionalHours)
	} else {
		hoursCanSkip := (float64(totalAttended) - 0.75*float64(totalHeld)) / 0.75
		totalInfo["hours_can_skip"] = int(hoursCanSkip)
	}

	result := map[string]interface{}{
		"roll_number":       rollNumber,
		"attendance_summary": attendanceSummary,
		"subjectwise_summary": subjectwiseSummary,
		"total_info":        totalInfo,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(resultJSON), nil
}
