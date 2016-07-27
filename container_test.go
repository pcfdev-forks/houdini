package houdini_test

import (
	"bytes"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var container garden.Container

	BeforeEach(func() {
		var err error
		container, err = backend.Create(garden.ContainerSpec{})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(backend.Destroy(container.Handle())).To(Succeed())
	})

	Describe("Streaming", func() {
		var destinationContainer garden.Container

		AfterEach(func() {
			Expect(backend.Destroy(destinationContainer.Handle())).To(Succeed())
		})

		It("can stream to and from container", func() {
			process, err := container.Run(garden.ProcessSpec{
				Path: "sh",
				Args: []string{
					"-exc",
					`
							touch a
							touch b
							mkdir foo/
							touch foo/in-foo-a
							touch foo/in-foo-b
						`,
				},
			}, garden.ProcessIO{
				Stdout: GinkgoWriter,
				Stderr: GinkgoWriter,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(process.Wait()).To(Equal(0))

			out, err := container.StreamOut(garden.StreamOutSpec{
				Path: ".",
			})
			Expect(err).ToNot(HaveOccurred())

			outBytes, err := ioutil.ReadAll(out)
			data := ioutil.NopCloser(bytes.NewReader(outBytes))

			Expect(backend.Destroy(container.Handle())).To(Succeed())

			destinationContainer, err = backend.Create(garden.ContainerSpec{})
			Expect(err).ToNot(HaveOccurred())

			err = destinationContainer.StreamIn(garden.StreamInSpec{
				Path:      ".",
				TarStream: data,
			})
			Expect(err).ToNot(HaveOccurred())

			nothing := make([]byte, 1)
			n, err := out.Read(nothing)
			Expect(n).To(Equal(0))
			Expect(err).To(Equal(io.EOF))

			checkTree, err := destinationContainer.Run(garden.ProcessSpec{
				Path: "sh",
				Args: []string{
					"-exc",
					`
							find .
							test -e a
							test -e b
							test -e foo/in-foo-a
							test -e foo/in-foo-b
						`,
				},
			}, garden.ProcessIO{
				Stdout: GinkgoWriter,
				Stderr: GinkgoWriter,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(checkTree.Wait()).To(Equal(0))
		})
	})
})
