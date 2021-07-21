package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func getArtifactsForJob(workflowID, jobName string) (string, error) {
	u := fmt.Sprintf("https://circleci.com/api/v2/workflow/%v/job", workflowID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	type JobsResponse struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			ProjectSlug string `json:"project_slug"`
			JobNumber   int    `json:"job_number"`
		} `json:"items"`
	}

	jobs := &JobsResponse{}
	err = json.Unmarshal(b, jobs)
	if err != nil {
		return "", err
	}

	num := 0
	slug := ""
	for _, j := range jobs.Items {
		if j.Name == jobName {
			num = j.JobNumber
			slug = j.ProjectSlug
		}
	}

	if slug == "" {
		return "", errors.New("no job found for name " + jobName)
	}

	u = fmt.Sprintf("https://circleci.com/api/v2/project/%v/%v/artifacts", slug, num)
	req, err = http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	b, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	type ArtifactsResponse struct {
		Items []struct {
			Path string `json:"path"`
			URL  string `json:"url"`
		} `json:"items"`
	}

	artifacts := &ArtifactsResponse{}
	err = json.Unmarshal(b, artifacts)
	if err != nil {
		return "", err
	}

	for _, a := range artifacts.Items {
		if strings.HasSuffix(a.Path, "tar.gz") {
			return a.URL, nil
		}
	}

	return "", errors.New("couldn't find artifact")
}
