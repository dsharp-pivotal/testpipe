package main_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var (
		pipelinePath string

		configFilePath string
		tmpDir         string
		someResourceDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "testpipe")
		Expect(err).NotTo(HaveOccurred())

		pipelineFile, err := ioutil.TempFile(tmpDir, "pipeline.yml")
		Expect(err).NotTo(HaveOccurred())
		defer pipelineFile.Close()

		pipelinePath = pipelineFile.Name()

		pipelineConfig := `---
jobs:
- name: some-job
  plan:
  - get: a-resource
  - task: some-task
    config:
      run:
        path: a-resource/some-command
`

		_, err = io.Copy(pipelineFile, strings.NewReader(pipelineConfig))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("exits successfully", func() {
		cmd := exec.Command(cmdPath, "-p", pipelinePath)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))
	})

	Context("when the pipeline refers to a task which is defined elsewhere", func() {
		BeforeEach(func() {
			resourcesDir, err := ioutil.TempDir(tmpDir, "resources")
			Expect(err).NotTo(HaveOccurred())

			someResourceDir = filepath.Join(resourcesDir, "some-resource")
			err = os.MkdirAll(someResourceDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			testpipeConfig := fmt.Sprintf(`---
resource_map:
  some-resource: %s
  fixture-resource: fixtures/fixture-resource`, someResourceDir)

			testpipeConfigFile, err := ioutil.TempFile(tmpDir, "testpipe-config.yml")
			Expect(err).NotTo(HaveOccurred())
			defer testpipeConfigFile.Close()

			configFilePath = testpipeConfigFile.Name()

			_, err = io.Copy(testpipeConfigFile, strings.NewReader(testpipeConfig))
			Expect(err).NotTo(HaveOccurred())

			taskPath := filepath.Join(someResourceDir, "task.yml")

			taskConfig := `---
inputs:
- name: some-resource
params:
  some_param:
run:
  path: some-resource/scripts/executable.sh
`

			err = ioutil.WriteFile(taskPath, []byte(taskConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			pipelineConfig := `---
jobs:
- name: some-job
  plan:
  - get: some-resource
  - get: fixture-resource
  - task: some-task
    params:
      some_param: A
    file: some-resource/task.yml
`

			err = ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath, "-c", configFilePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})

		Context("when the script is not executable", func() {
			BeforeEach(func() {
				taskConfig := `---
inputs:
- name: fixture-resource
params:
  some_param:
run:
  path: fixture-resource/scripts/not_executable.sh
`
			taskPath := filepath.Join(someResourceDir, "task.yml")
				err := ioutil.WriteFile(taskPath, []byte(taskConfig), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("checks that the script is executable", func() {
				cmd := exec.Command(cmdPath, "-p", pipelinePath, "-c", configFilePath)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
			})
		})

		Context("when the script is missing", func() {
			BeforeEach(func() {
				taskConfig := `---
inputs:
- name: fixture-resource
params:
  some_param:
run:
  path: fixture-resource/scripts/not_present.sh
`
			taskPath := filepath.Join(someResourceDir, "task.yml")
				err := ioutil.WriteFile(taskPath, []byte(taskConfig), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("checks that the script is present", func() {
				cmd := exec.Command(cmdPath, "-p", pipelinePath, "-c", configFilePath)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
			})
		})
	})

	Context("when the pipeline specifies params that a task does not require", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
    params:
      some_param: A
      some_other_param: B
    config:
      inputs:
      - name: a-resource
      params:
        some_param:
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("Extra params that should be removed"))
			Eventually(session.Err).Should(gbytes.Say("some_other_param"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when the pipeline does not specify params that a task requires", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
    config:
      params:
        some_param:
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("Missing params that should be added"))
			Eventually(session.Err).Should(gbytes.Say("some_param"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when the pipeline does not specify a resource that a task requires", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
    config:
      inputs:
      - name: a-resource
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("Task invocation is missing resources"))
			Eventually(session.Err).Should(gbytes.Say("a-resource"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when the pipeline uses input_mapping to specify a resource that a task requires", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    input_mapping:
      a-resource: some-resource
    config:
      inputs:
      - name: a-resource
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when the pipeline uses output_mapping", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: upstream-task
    output_mapping:
      some-resource: a-resource
    config:
      run:
        path: some-command
  - task: some-task
    config:
      inputs:
      - name: a-resource
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when the pipeline defines a task inline", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - get: a-resource
  - task: some-task
    params:
      some_param: A
    config:
      inputs:
      - name: a-resource
      params:
        some_param:
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when a task provides as an output the required input of another task", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-upstream-task
    config:
      outputs:
      - name: a-resource
      run:
        path: some-command
  - task: some-task
    config:
      inputs:
      - name: a-resource
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when a task provides as an output the required input of another task but the task is defined downstream", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
    config:
      inputs:
      - name: a-resource
      run:
        path: some-command
  - task: some-downstream-task
    config:
      outputs:
      - name: a-resource
      run:
        path: some-command
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("Task invocation is missing resources"))
			Eventually(session.Err).Should(gbytes.Say("a-resource"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when a task is configured with a file but no config is given", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
    file: a-resource/task.yml
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("failed to load a-resource/task.yml; no config provided"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when a get renames a resource which is the source of its config", func() {
		BeforeEach(func() {
			resourcesDir, err := ioutil.TempDir(tmpDir, "resources")
			Expect(err).NotTo(HaveOccurred())

			someResourceDir = filepath.Join(resourcesDir, "some-resource")
			err = os.MkdirAll(someResourceDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			testpipeConfig := fmt.Sprintf(`---
resource_map:
  some-resource: %s`, someResourceDir)

			testpipeConfigFile, err := ioutil.TempFile(tmpDir, "testpipe-config.yml")
			Expect(err).NotTo(HaveOccurred())
			defer testpipeConfigFile.Close()

			configFilePath = testpipeConfigFile.Name()

			_, err = io.Copy(testpipeConfigFile, strings.NewReader(testpipeConfig))
			Expect(err).NotTo(HaveOccurred())

			taskPath := filepath.Join(someResourceDir, "task.yml")

			taskConfig := `---
inputs: []
params: {}
`

			err = ioutil.WriteFile(taskPath, []byte(taskConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - get: a-resource
    resource: some-resource
  - task: some-task
    config:
      run:
        path: some-command
`)

			err = ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath, "-c", configFilePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("when a get renames a resource which is the source of its config and has invalid config", func() {
		BeforeEach(func() {
			resourcesDir, err := ioutil.TempDir(tmpDir, "resources")
			Expect(err).NotTo(HaveOccurred())

			someResourceDir = filepath.Join(resourcesDir, "some-resource")
			err = os.MkdirAll(someResourceDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			testpipeConfig := fmt.Sprintf(`---
resource_map:
  what-resource: %s`, someResourceDir)

			testpipeConfigFile, err := ioutil.TempFile(tmpDir, "testpipe-config.yml")
			Expect(err).NotTo(HaveOccurred())
			defer testpipeConfigFile.Close()

			configFilePath = testpipeConfigFile.Name()

			_, err = io.Copy(testpipeConfigFile, strings.NewReader(testpipeConfig))
			Expect(err).NotTo(HaveOccurred())

			taskPath := filepath.Join(someResourceDir, "task.yml")

			taskConfig := `---
inputs: []
params: {}
`

			err = ioutil.WriteFile(taskPath, []byte(taskConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - get: a-resource
    resource: some-resource
  - task: some-task
    file: a-resource/task.yml
`)

			err = ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath, "-c", configFilePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("failed to find path for task: a-resource/task.yml"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when the pipeline defines an empty task", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("some-job/some-task is missing a definition"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when a task has no path to run", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jobs:
- name: some-job
  plan:
  - task: some-task
    config:
      run:
        path:
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			cmd := exec.Command(cmdPath, "-p", pipelinePath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("some-job/some-task is missing a path"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when jobs is misspelled", func() {
		BeforeEach(func() {
			pipelineConfig := fmt.Sprintf(`---
jorbs:
- name: some-job
  plan:
  - task: some-task
`)

			err := ioutil.WriteFile(pipelinePath, []byte(pipelineConfig), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error", func() {
			session := RunTestpipeCommand(pipelinePath)

			Eventually(session.Err).Should(gbytes.Say("no jobs in pipeline"))

			Eventually(session).Should(gexec.Exit(1))
		})
	})
})

func RunTestpipeCommand(pipelinePath string) *gexec.Session {
	cmd := exec.Command(cmdPath, "-p", pipelinePath)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}
