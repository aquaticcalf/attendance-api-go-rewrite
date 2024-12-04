package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"test"
)

func getAttendance(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	password := r.URL.Query().Get("password")

	if studentID == "" || password == "" {
		http.Error(w, `{"error": "Missing student_id or password"}`, http.StatusBadRequest)
		return
	}

	attendanceData, err := test.FetchAttendance(studentID, password)
	if err != nil {
		http.Error(w, `{"error": "Failed to fetch attendance data"}`, http.StatusInternalServerError)
		return
	}

	var result interface{}
	err = json.Unmarshal([]byte(attendanceData), &result)
	if err != nil {
		http.Error(w, `{"error": "Failed to parse attendance data"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func compareAttendance(w http.ResponseWriter, r *http.Request) {
	var studentsData []map[string]interface{}
	var subjectPoints = make(map[string][]map[string]interface{})

	err := json.NewDecoder(r.Body).Decode(&studentsData)
	if err != nil || len(studentsData) == 0 {
		http.Error(w, `{"error": "Invalid input. Expecting a list of student credentials."}`, http.StatusBadRequest)
		return
	}

	for _, student := range studentsData {
		studentID, studentIDOk := student["student_id"].(string)
		password, passwordOk := student["password"].(string)

		if !studentIDOk || !passwordOk {
			student["error"] = "Missing student_id or password"
			continue
		}

		attendanceData, err := test.FetchAttendance(studentID, password)
		if err != nil {
			student["error"] = "Failed to fetch attendance data"
			continue
		}

		var attendance map[string]interface{}
		err = json.Unmarshal([]byte(attendanceData), &attendance)
		if err != nil {
			student["error"] = "Failed to parse attendance data"
			continue
		}

		totalInfo, totalInfoOk := attendance["total_info"].(map[string]interface{})
		if !totalInfoOk {
			student["error"] = "Missing total_info in attendance data"
			continue
		}

		totalAttended := totalInfo["total_attended"].(float64)
		totalHeld := totalInfo["total_held"].(float64)
		totalPercentage := totalInfo["total_percentage"].(string)
		additionalHoursNeeded := totalInfo["additional_hours_needed"].(float64)
		hoursCanSkip := totalInfo["hours_can_skip"].(float64)

		subjectSummary, subjectSummaryOk := attendance["subjectwise_summary"].([]interface{})
		if subjectSummaryOk {
			for _, subjectItem := range subjectSummary {
				subject, subjectOk := subjectItem.(map[string]interface{})
				if subjectOk {
					subjectName := subject["subject_name"].(string)
					percentage := strings.TrimSuffix(subject["percentage"].(string), "%")
					percentageFloat, _ := strconv.ParseFloat(percentage, 64)

					if _, exists := subjectPoints[subjectName]; !exists {
						subjectPoints[subjectName] = []map[string]interface{}{}
					}

					subjectPoints[subjectName] = append(subjectPoints[subjectName], map[string]interface{}{
						"student_id": studentID,
						"percentage": percentageFloat,
					})
				}
			}
		}

		student["total_attended"] = totalAttended
		student["total_held"] = totalHeld
		student["total_percentage"] = totalPercentage
		student["hours_status"] = hoursCanSkip
		if hoursCanSkip <= 0 {
			student["hours_status"] = -additionalHoursNeeded
		}
	}

	subjectPointsSummary := make(map[string]interface{})
	for subject, scores := range subjectPoints {
		maxPercentage := float64(0)
		topStudents := []string{}

		for _, score := range scores {
			percentage := score["percentage"].(float64)
			if percentage > maxPercentage {
				maxPercentage = percentage
				topStudents = []string{score["student_id"].(string)}
			} else if percentage == maxPercentage {
				topStudents = append(topStudents, score["student_id"].(string))
			}
		}

		subjectPointsSummary[subject] = map[string]interface{}{
			"max_percentage": maxPercentage,
			"top_students":   topStudents,
		}
	}

	comparisonSummary := map[string]interface{}{
		"students":            studentsData,
		"subject_points_summary": subjectPointsSummary,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comparisonSummary)
}

func calculateAttendanceAfterSkip(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	password := r.URL.Query().Get("password")
	skipHoursStr := r.URL.Query().Get("hours")

	if studentID == "" || password == "" || skipHoursStr == "" {
		http.Error(w, `{"error": "Missing student_id, password, or hours"}`, http.StatusBadRequest)
		return
	}

	skipHours, err := strconv.Atoi(skipHoursStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid hours parameter"}`, http.StatusBadRequest)
		return
	}

	attendanceData, err := test.FetchAttendance(studentID, password)
	if err != nil {
		http.Error(w, `{"error": "Failed to fetch attendance data"}`, http.StatusInternalServerError)
		return
	}

	var attendance map[string]interface{}
	err = json.Unmarshal([]byte(attendanceData), &attendance)
	if err != nil {
		http.Error(w, `{"error": "Failed to parse attendance data"}`, http.StatusInternalServerError)
		return
	}

	totalInfo := attendance["total_info"].(map[string]interface{})
	totalAttended := totalInfo["total_attended"].(float64)
	totalHeld := totalInfo["total_held"].(float64)

	newTotalHeld := totalHeld + float64(skipHours)
	newPercentage := (totalAttended / newTotalHeld) * 100

	status := "needs to attend more"
	var additionalHoursNeeded int
	if newPercentage >= 75 {
		status = "safe to skip"
		hoursCanSkip := int((totalAttended - 0.75*newTotalHeld) / 0.75)
		result := map[string]interface{}{
			"original_attendance_percentage": totalInfo["total_percentage"].(string),
			"new_attendance_percentage":      newPercentage,
			"status":                         status,
			"hours_can_skip_after":           hoursCanSkip,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	} else {
		additionalHoursNeeded = int((0.75*newTotalHeld - totalAttended) / (1 - 0.75))
	}

	result := map[string]interface{}{
		"original_attendance_percentage": totalInfo["total_percentage"].(string),
		"new_attendance_percentage":      newPercentage,
		"status":                         status,
		"additional_hours_needed_after":  additionalHoursNeeded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	http.HandleFunc("/attendance", getAttendance)
	http.HandleFunc("/compare", compareAttendance)
	http.HandleFunc("/skip", calculateAttendanceAfterSkip)

	log.Println("Server started on :5000")
	log.Fatal(http.ListenAndServe(":5000", nil))
}
