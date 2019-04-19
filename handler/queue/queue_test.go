package queue

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/tomatool/tomato/handler/queue/mocks"
)

func TestPublishMessage(t *testing.T) {

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	q := mocks.NewMockResource(ctrl)
	h := New(map[string]Resource{"q": q})

	for _, tc := range []struct {
		name string

		resourceName string
		target       string
		payload      string

		err string
	}{
		{
			name: "invalid resource name",
			err:  "not found",
		},
		{
			name:         "",
			resourceName: "q",
			target:       "abc",
			payload:      `{"":""}`,
		},
	} {
		if tc.err == "" {
			q.EXPECT().Publish(tc.target, []byte(tc.payload)).Return(nil)
		}

		err := h.publishMessage(tc.resourceName, tc.target, &gherkin.DocString{Content: tc.payload})
		if err != nil {
			assert.Contains(t, err.Error(), tc.err)
		}
		if tc.err != "" {
			assert.Error(t, err)
		}
	}
}

func TestListenMessage(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	q := mocks.NewMockResource(ctrl)
	h := New(map[string]Resource{"q": q})

	for _, tc := range []struct {
		name string

		resourceName string
		target       string

		err string
	}{
		{
			name: "invalid resource name",
			err:  "not found",
		},
		{
			name:         "",
			resourceName: "q",
			target:       "abc",
		},
	} {
		if tc.err == "" {
			q.EXPECT().Listen(tc.target).Return(nil)
		}

		err := h.listenMessage(tc.resourceName, tc.target)
		if err != nil {
			assert.Contains(t, err.Error(), tc.err)
		}
		if tc.err != "" {
			assert.Error(t, err)
		}

	}
}

func TestCountMessage(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	q := mocks.NewMockResource(ctrl)
	h := New(map[string]Resource{"q": q})

	for _, tc := range []struct {
		name string

		resourceName string
		target       string
		count        int

		fetchValue [][]byte
		fetchErr   error

		err string
	}{
		{
			name:         "count match",
			resourceName: "q",
			target:       "ex:rk",

			count:      2,
			fetchValue: [][]byte{{}, {}},
		},
		{
			name:         "count mismatch",
			resourceName: "q",
			target:       "another-target",

			count:      1,
			fetchValue: [][]byte{{}, {}},
		},
		{
			name:         "fetch error",
			resourceName: "q",
			target:       "my-topic",

			fetchErr: errors.New("failed to fetch message from queue"),
			err:      "failed to fetch message from queue",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {

			q.EXPECT().Fetch(tc.target).Return(tc.fetchValue, tc.fetchErr)

			err := h.countMessage(tc.resourceName, tc.target, tc.count)
			if tc.err != "" {
				assert.EqualError(t, err, tc.err)
			}
		})
	}
}
