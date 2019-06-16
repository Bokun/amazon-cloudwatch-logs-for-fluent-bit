// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package main

import (
	"C"
	"fmt"
	"unsafe"

	"time"

	"github.com/awslabs/amazon-cloudwatch-logs-for-fluent-bit/cloudwatch"
	"github.com/fluent/fluent-bit-go/output"

	"github.com/sirupsen/logrus"
)
import "strings"

var (
	cloudwatchLogs *cloudwatch.OutputPlugin
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "cloudwatch", "AWS CloudWatch Fluent Bit Plugin!")
}

func getConfiguration(ctx unsafe.Pointer) cloudwatch.OutputPluginConfig {
	config := cloudwatch.OutputPluginConfig{}
	config.LogGroupName = output.FLBPluginConfigKey(ctx, "log_group_name")
	logrus.Infof("[cloudwatch] plugin parameter log_group = '%s'\n", config.LogGroupName)
	config.LogStreamPrefix = output.FLBPluginConfigKey(ctx, "log_stream_prefix")
	logrus.Infof("[cloudwatch] plugin parameter log_stream_prefix = '%s'\n", config.LogStreamPrefix)
	config.LogStreamName = output.FLBPluginConfigKey(ctx, "log_stream_name")
	logrus.Infof("[cloudwatch] plugin parameter log_stream = '%s'\n", config.LogStreamName)
	config.Region = output.FLBPluginConfigKey(ctx, "region")
	logrus.Infof("[cloudwatch] plugin parameter = '%s'\n", config.Region)
	config.LogKey = output.FLBPluginConfigKey(ctx, "log_key")
	logrus.Infof("[cloudwatch] plugin parameter log_key = '%s'\n", config.LogKey)
	config.RoleARN = output.FLBPluginConfigKey(ctx, "role_arn")
	logrus.Infof("[cloudwatch] plugin parameter role_arn = '%s'\n", config.RoleARN)
	config.AutoCreateGroup = getBoolParam(ctx, "auto_create_group", false)
	logrus.Infof("[cloudwatch] plugin parameter auto_create_group = '%v'\n", config.AutoCreateGroup)

	return config
}

func getBoolParam(ctx unsafe.Pointer, param string, defaultVal bool) bool {
	val := strings.ToLower(output.FLBPluginConfigKey(ctx, param))
	if val == "true" {
		return true
	} else if val == "false" {
		return false
	} else {
		return defaultVal
	}
}

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
	config := getConfiguration(ctx)
	err := config.Validate()
	if err != nil {
		logrus.Error(err)
		return output.FLB_ERROR
	}

	cloudwatchLogs, err = cloudwatch.NewOutputPlugin(config)
	if err != nil {
		logrus.Error(err)
		return output.FLB_ERROR
	}
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var count int
	var ret int
	var ts interface{}
	var record map[interface{}]interface{}

	// Create Fluent Bit decoder
	dec := output.NewDecoder(data, int(length))

	fluentTag := C.GoString(tag)
	logrus.Debugf("[firehose] Found logs with tag: %s\n", fluentTag)

	for {
		// Extract Record
		ret, ts, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}

		var timestamp time.Time
		switch tts := ts.(type) {
		case output.FLBTime:
			timestamp = tts.Time
		case uint64:
			// From our observation, when ts is of type uint64 it appears to
			// be the amount of seconds since unix epoch.
			timestamp = time.Unix(int64(tts), 0)
		default:
			timestamp = time.Now()
		}

		retCode := cloudwatchLogs.AddEvent(fluentTag, record, timestamp)
		if retCode != output.FLB_OK {
			return retCode
		}
		count++
	}
	err := cloudwatchLogs.Flush(fluentTag)
	if err != nil {
		fmt.Println(err)
		// TODO: Better error handling
		return output.FLB_RETRY
	}

	logrus.Debugf("[firehose] Processed %d events with tag %s\n", count, fluentTag)

	// Return options:
	//
	// output.FLB_OK    = data have been processed.
	// output.FLB_ERROR = unrecoverable error, do not try this again.
	// output.FLB_RETRY = retry to flush later.
	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
