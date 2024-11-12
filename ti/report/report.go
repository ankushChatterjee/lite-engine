// Copyright 2022 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package report

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/harness/lite-engine/api"
	tiCfg "github.com/harness/lite-engine/ti/config"
	"github.com/harness/lite-engine/ti/report/parser/junit"
	"github.com/harness/ti-client/types"
	"github.com/sirupsen/logrus"
)

func ParseAndUploadTests(ctx context.Context, report api.TestReport, workDir, stepID string, log *logrus.Logger, start time.Time, tiConfig *tiCfg.Cfg, envs map[string]string) error {
	if report.Kind != api.Junit {
		return fmt.Errorf("unknown report type: %s", report.Kind)
	}

	if len(report.Junit.Paths) == 0 {
		return nil
	}

	// Append working dir to the paths. In k8s, we specify the workDir in the YAML but this is
	// needed in case of VMs.
	for idx, p := range report.Junit.Paths {
		if p[0] != '~' && p[0] != '/' && p[0] != '\\' {
			if !strings.HasPrefix(p, workDir) {
				report.Junit.Paths[idx] = filepath.Join(workDir, p)
			}
		}
	}

	tests := junit.ParseTests(report.Junit.Paths, log, envs)
	if len(tests) == 0 {
		return nil
	}

	startTime := time.Now()
	logrus.WithContext(ctx).Infoln(fmt.Sprintf("Starting TI service request to write report for step %s", stepID))
	c := tiConfig.GetClient()
	if err := c.Write(ctx, stepID, strings.ToLower(report.Kind.String()), tests); err != nil {
		return err
	}
	logrus.WithContext(ctx).Infoln(fmt.Sprintf("Completed TI service request to write report for step %s, took %.2f seconds", stepID, time.Since(startTime).Seconds()))
	log.Infoln(fmt.Sprintf("Successfully collected test reports in %s time", time.Since(start)))
	return nil
}

func SaveReportSummaryToOutputs(ctx context.Context, tiConfig *tiCfg.Cfg, stepID string, outputs map[string]string, log *logrus.Logger) error {
	tiClient := tiConfig.GetClient()
	sumamryRequest := types.SummaryRequest{
		AllStages:  true,
		OrgID:      tiConfig.GetOrgID(),
		ProjectID:  tiConfig.GetProjectID(),
		PipelineID: tiConfig.GetPipelineID(),
		BuildID:    tiConfig.GetBuildID(),
		StageID:    tiConfig.GetStageID(),
		StepID:     stepID,
		ReportType: "junit",
	}
	response, err := tiClient.Summary(ctx, sumamryRequest)
	if err != nil {
		return nil
	}
	// write to output file
	log.Infof(fmt.Sprintf("Number of tests run: %d", response.TotalTests))
	outputs["total_tests"] = fmt.Sprintf("%d", response.TotalTests)
	outputs["successful_tests"] = fmt.Sprintf("%d", response.SuccessfulTests)
	outputs["failed_tests"] = fmt.Sprintf("%d", response.FailedTests)
	outputs["skipped_tests"] = fmt.Sprintf("%d", response.SkippedTests)
	outputs["duration_ms"] = fmt.Sprintf("%d", response.TimeMs)
	return nil
}
