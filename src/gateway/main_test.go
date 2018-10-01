package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hello world", func() {
	It("Can be true", func() {
		Expect(true).To(BeTrue())
	})
})
