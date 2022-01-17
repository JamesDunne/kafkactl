package k8s

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/deviceinsight/kafkactl/internal"
	"github.com/deviceinsight/kafkactl/output"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var KafkaCtlVersion string

type Operation struct {
	context internal.ClientContext
}

func (operation *Operation) initialize() error {

	if !operation.context.Kubernetes.Enabled {
		return errors.Errorf("context is not a kubernetes context: %s", operation.context.Name)
	}

	if operation.context.Kubernetes.KubeContext == "" {
		return errors.Errorf("context has no kubernetes context set: contexts.%s.kubernetes.kubeContext", operation.context.Name)
	}

	if operation.context.Kubernetes.Namespace == "" {
		return errors.Errorf("context has no kubernetes namespace set: contexts.%s.kubernetes.namespace", operation.context.Name)
	}

	return nil
}

func (operation *Operation) Attach() error {

	var err error

	if operation.context, err = internal.CreateClientContext(); err != nil {
		return err
	}

	if err := operation.initialize(); err != nil {
		return err
	}

	exec := newExecutor(operation.context, &ShellRunner{})

	podEnvironment := parsePodEnvironment(operation.context)

	return exec.Run("ubuntu", "bash", nil, podEnvironment)
}

func (operation *Operation) TryRun(cmd *cobra.Command, args []string) bool {

	var err error

	if operation.context, err = internal.CreateClientContext(); err != nil {
		return false
	}

	if !operation.context.Kubernetes.Enabled {
		return false
	}

	if err := operation.Run(cmd, args); err != nil {
		output.Fail(err)
	}
	return true
}

func (operation *Operation) Run(cmd *cobra.Command, args []string) error {

	if err := operation.initialize(); err != nil {
		return err
	}

	exec := newExecutor(operation.context, &ShellRunner{})

	kafkaCtlCommand := parseCompleteCommand(cmd, []string{})
	kafkaCtlFlags, err := parseFlags(cmd)
	if err != nil {
		return err
	}

	podEnvironment := parsePodEnvironment(operation.context)

	kafkaCtlCommand = append(kafkaCtlCommand, args...)
	kafkaCtlCommand = append(kafkaCtlCommand, kafkaCtlFlags...)

	return exec.Run("scratch", "/kafkactl", kafkaCtlCommand, podEnvironment)
}

func parseFlags(cmd *cobra.Command) ([]string, error) {
	var flags []string
	var err error

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if err == nil && flag.Changed {
			if flag.Value.Type() == "intSlice" || flag.Value.Type() == "int32Slice" {
				var intArray []int
				intArray, err = parseIntArray(flag.Value.String())
				if err == nil {
					for _, value := range intArray {
						flags = append(flags, fmt.Sprintf("--%s=%s", flag.Name, strconv.Itoa(value)))
					}
				}
			} else {
				flags = append(flags, fmt.Sprintf("--%s=%s", flag.Name, flag.Value.String()))
			}
		}
	})
	return flags, err
}

func parseIntArray(array string) ([]int, error) {
	var ints []int
	err := json.Unmarshal([]byte(array), &ints)
	return ints, err
}

func parseCompleteCommand(cmd *cobra.Command, found []string) []string {
	if cmd.Parent() == nil {
		return found
	}
	newCommand := []string{cmd.Name()}
	found = append(newCommand, found...)
	return parseCompleteCommand(cmd.Parent(), found)
}

func parsePodEnvironment(context internal.ClientContext) []string {

	var env []string

	env = appendStrings(env, "BROKERS", context.Brokers)
	env = appendBool(env, "TLS_ENABLED", context.TLS.Enabled)
	env = appendStringIfDefined(env, "TLS_CA", context.TLS.CA)
	env = appendStringIfDefined(env, "TLS_CERT", context.TLS.Cert)
	env = appendStringIfDefined(env, "TLS_CERTKEY", context.TLS.CertKey)
	env = appendBool(env, "TLS_INSECURE", context.TLS.Insecure)
	env = appendBool(env, "SASL_ENABLED", context.Sasl.Enabled)
	env = appendStringIfDefined(env, "SASL_USERNAME", context.Sasl.Username)
	env = appendStringIfDefined(env, "SASL_PASSWORD", context.Sasl.Password)
	env = appendStringIfDefined(env, "SASL_MECHANISM", context.Sasl.Mechanism)
	env = appendStringIfDefined(env, "REQUESTTIMEOUT", context.RequestTimeout.String())
	env = appendStringIfDefined(env, "CLIENTID", context.ClientID)
	env = appendStringIfDefined(env, "KAFKAVERSION", context.KafkaVersion.String())
	env = appendStringIfDefined(env, "AVRO_SCHEMAREGISTRY", context.AvroSchemaRegistry)
	env = appendStringIfDefined(env, "DEFAULTPARTITIONER", context.DefaultPartitioner)

	return env
}

func appendStrings(env []string, key string, value []string) []string {
	return append(env, fmt.Sprintf("%s=%s", key, strings.Join(value, " ")))
}

func appendBool(env []string, key string, value bool) []string {
	if value {
		return append(env, fmt.Sprintf("%s=%t", key, value))
	}
	return env
}

func appendStringIfDefined(env []string, key string, value string) []string {
	if value != "" {
		return append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}
