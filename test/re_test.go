package test

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

func TestRun(t *testing.T) {
	//goland:noinspection SpellCheckingInspection
	testCases := []struct {
		give string
	}{
		{"normal"},
		{"kanokari"},
		{"isekai"},
		{"eureka_ao"},
	}

	for _, tt := range testCases {
		t.Run(tt.give, func(t *testing.T) {
			re.FileSystem = afero.NewMemMapFs()
			defer func() { re.FileSystem = afero.NewOsFs() }()

			testDataPath := filepath.Join("testdata", tt.give+".csv")
			testData := readTestData(t, testDataPath)

			basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")

			setup(t, basePath, testData)

			re.Run(basePath, strings.NewReader("Y\n"))

			for _, datum := range testData {
				f0, err := re.FileSystem.Open(filepath.Join(basePath, datum.wantSubtitle))
				require.NoError(t, err, datum.wantSubtitle)

				content, err := afero.ReadAll(f0)
				assert.NoError(t, err)
				assert.Equal(t, datum.content, string(content))

				f1, err := re.FileSystem.Open(filepath.Join(basePath, datum.giveMovie))
				require.NoError(t, err)

				content, err = afero.ReadAll(f1)
				assert.NoError(t, err)
				assert.Equal(t, datum.content, string(content))

				f2, err := re.FileSystem.Open(filepath.Join(basePath, datum.giveSubtitle))
				assert.Error(t, err, datum.giveSubtitle)
				assert.Contains(t, err.Error(), os.ErrNotExist.Error())
				assert.Nil(t, f2)
			}
		})
	}
}

func readTestData(t *testing.T, filepath string) []testDatum {
	f, err := os.Open(filepath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	r := csv.NewReader(f)

	var testData []testDatum
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		testData = append(testData, testDatum{
			giveMovie:    record[0],
			giveSubtitle: record[1],
			wantSubtitle: record[2],
			content:      record[3],
		})
	}
	return testData
}

type testDatum struct {
	giveMovie    string
	giveSubtitle string
	wantSubtitle string
	content      string
}

func setup(t *testing.T, prefix string, testData []testDatum) {
	for _, datum := range testData {
		err := afero.WriteFile(re.FileSystem, filepath.Join(prefix, datum.giveMovie), []byte(datum.content), 0644)
		require.NoError(t, err)

		err = afero.WriteFile(re.FileSystem, filepath.Join(prefix, datum.giveSubtitle), []byte(datum.content), 0644)
		require.NoError(t, err)
	}
}
