package filedb_test

import (
	"ccoms/pkg/config"
	"ccoms/pkg/filedb"
	"fmt"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	fdb, err := filedb.New(path.Join(config.DEVDATA, "filedb/test.log"))
	require.Nil(t, err)

	txt := "{this a hi}"
	err = fdb.WriteLine(txt + "\n")
	require.Nil(t, err)

	s, err := fdb.ReadLastLine()
	require.Nil(t, err)
	require.Equal(t, txt, s)
}

func TestFollow(t *testing.T) {
	fdb, err := filedb.New(path.Join(config.DEVDATA, "filedb/test.log"))
	require.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	ch := make(chan string, 64)
	go func() {
		defer wg.Done()

		for i := 0; i < 100; i++ {
			err = fdb.WriteLine(fmt.Sprintf("hi %d\n", i))
			if err != nil {
				return
			}
			time.Sleep(10 * time.Microsecond)
		}
	}()

	go func() {
		err = fdb.Tailf(ch)
		if err != nil {
			return
		}
	}()

	go func() {
		defer wg.Done()

		for {
			fmt.Println("recv:", <-ch)
		}
	}()

	wg.Wait()
}

func TestReadLastLine(t *testing.T) {
	fdb, err := filedb.New(path.Join(config.DEVDATA, "filedb/test.log"))
	require.Nil(t, err)
	line, err := fdb.ReadLastLine()
	require.Nil(t, err)
	fmt.Println(line)
}

func TestReadFirstLine(t *testing.T) {
	fdb, err := filedb.New(path.Join(config.DEVDATA, "filedb/test.log"))
	require.Nil(t, err)
	line, err := fdb.ReadFirstLine()
	require.Nil(t, err)
	fmt.Println(line)
}

func BenchmarkWrite(b *testing.B) {
	fdb, err := filedb.New(path.Join(config.DEVDATA, "filedb/test.log"))
	require.Nil(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fdb.WriteLine("vFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTMvFFDUPCTQVYuzFEhgjxPmHnwLxswVNPjOSNbMk6zDA3qPltQVuuTPcJXHpv31eTM\n")
	}
}
