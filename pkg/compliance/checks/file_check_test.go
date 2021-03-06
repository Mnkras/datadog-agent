// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

// +build !windows

package checks

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/compliance"
	"github.com/DataDog/datadog-agent/pkg/compliance/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFileCheck(t *testing.T) {
	type setupFunc func(t *testing.T, bc baseCheck) *fileCheck
	type validateFunc func(t *testing.T, kv compliance.KVMap)

	setupFile := func(file *compliance.File) setupFunc {
		return func(t *testing.T, bc baseCheck) *fileCheck {
			return &fileCheck{
				baseCheck: bc,
				file:      file,
			}
		}
	}

	tests := []struct {
		name     string
		setup    setupFunc
		validate validateFunc
	}{
		{
			name: "permissions",
			setup: func(t *testing.T, bc baseCheck) *fileCheck {
				dir := os.TempDir()
				fileName := fmt.Sprintf("test-permissions-file-check-%d.dat", time.Now().Unix())
				filePath := path.Join(dir, fileName)
				f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
				defer f.Close()
				assert.NoError(t, err)

				mapper := func(filePath string) string { return path.Join(dir, filePath) }
				file := &compliance.File{
					Path: fileName,
					Report: compliance.Report{
						{
							Property: "permissions",
							Kind:     compliance.PropertyKindAttribute,
						},
					},
				}
				return &fileCheck{
					baseCheck:  bc,
					pathMapper: mapper,
					file:       file,
				}
			},
			validate: func(t *testing.T, kv compliance.KVMap) {
				assert.Equal(t, compliance.KVMap{
					"permissions": "644",
				}, kv)
			},
		},
		{
			name: "owner root",
			setup: setupFile(&compliance.File{
				Path: "/tmp",
				Report: compliance.Report{
					{
						Property: "owner",
						Kind:     compliance.PropertyKindAttribute,
					},
					{
						Property: "path",
						Kind:     compliance.PropertyKindAttribute,
					},
				},
			}),
			validate: func(t *testing.T, kv compliance.KVMap) {
				owner, ok := kv["owner"]
				assert.True(t, ok)
				parts := strings.SplitN(owner, ":", 2)
				assert.Equal(t, parts[0], "root")
				assert.Contains(t, []string{"root", "wheel"}, parts[1])
				assert.Equal(t, "/tmp", kv["path"])
			},
		},
		{
			name: "jsonquery log-driver",
			setup: setupFile(&compliance.File{
				Path: "./testdata/file/daemon.json",
				Report: compliance.Report{
					{
						// Need to use .[] syntax when attributes have - in their name
						Property: `.["log-driver"]`,
						Kind:     compliance.PropertyKindJSONQuery,
						As:       "log_driver",
					},
				},
			}),
			validate: func(t *testing.T, kv compliance.KVMap) {
				assert.Equal(t, compliance.KVMap{
					"log_driver": "json-file",
				}, kv)
			},
		},
		{
			name: "jsonquery experimental",
			setup: setupFile(&compliance.File{
				Path: "./testdata/file/daemon.json",
				Report: compliance.Report{
					{
						Property: ".experimental",
						Kind:     "jsonquery",
						As:       "experimental",
					},
				},
			}),
			validate: func(t *testing.T, kv compliance.KVMap) {
				assert.Equal(t, compliance.KVMap{
					"experimental": "false",
				}, kv)
			},
		},
		{
			name: "jsonquery ulimits",
			setup: setupFile(&compliance.File{
				Path: "./testdata/file/daemon.json",
				Report: compliance.Report{
					{
						Property: `.["default-ulimits"].nofile.Hard`,
						Kind:     "jsonquery",
						As:       "nofile_hard",
					},
				},
			}),
			validate: func(t *testing.T, kv compliance.KVMap) {
				assert.Equal(t, compliance.KVMap{
					"nofile_hard": "64000",
				}, kv)
			},
		},
		{
			name: "yamlquery pod",
			setup: setupFile(&compliance.File{
				Path: "./testdata/file/pod.yaml",
				Report: compliance.Report{
					{
						Property: ".apiVersion",
						Kind:     "yamlquery",
						As:       "apiVersion",
					},
				},
			}),
			validate: func(t *testing.T, kv compliance.KVMap) {
				assert.Equal(t, compliance.KVMap{
					"apiVersion": "v1",
				}, kv)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &mocks.Reporter{}
			fc := test.setup(t, newTestBaseCheck(reporter, checkKindFile))

			reporter.On(
				"Report",
				mock.AnythingOfType("*compliance.RuleEvent"),
			).Run(func(args mock.Arguments) {
				event := args.Get(0).(*compliance.RuleEvent)
				test.validate(t, event.Data)
			})
			defer reporter.AssertExpectations(t)

			err := fc.Run()
			assert.NoError(t, err)
		})
	}
}
