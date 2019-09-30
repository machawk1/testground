# Test Plans to be written for Test Ground MVP

The following test cases have been identified as the initial set of tests to implement using testground, with the goal of delivering a good characterization of the performance of IPFS in specific areas, while still being possible to deliver the tests within a 1 month implementation period.

Each Test Plan description contains:
- An overview of what we are looking to achieve with the test (roughly ~1 paragraph).
- What we are looking to expect to be able to optimize by running this test and therefore, a suggestion of what are The data points that must be gathered in order to assess if an improvement or regression has been made.-
- The test parameters. This include both Network Parameters (e.g. Number of Nodes) and Image Parameters (e.g. bucket_size, bitswap strategy, etc)
- Narrative that describes on how the network will set up itself (_Warm Up_ phase) and how the actors will play their multiple roles (in _Waves_).

## Test Plans

### 1. Chewing strategies for Large DataSets

IPFS supports an ever-growing set of ways in how a File or Files can be added to the Network (Regular IPFS Add, MFS Add, Sharding, Balanced DAG, Trickle DAG, File Store, URL Store). This test plan checks their performance

#### What is being optimized (min/max, reach)

- (Minimize) Memory used when performing each of the instructions
- (Minimize) Time spent time chewing the files/directories
- (Minimize) Time spent pinning/unpinning & garbage collection
- (Minimize) Time spent creating the Manifest files using URL and File Store
- (Minimize) Number of leftover and unreferenceable nodes from the MFS root when adding multiple files to MFS
- (Minimize) Waste when adding multiple files to an MFS tree (nodes that are no longer referenceable from the MFS tree due to graph updates)

#### Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - `N` - Number of nodes that are spawn for the test (from 10 to 1000000)
- **Image Parameters**
  - Single Image - The go-ipfs commit that is being tested

#### Narrative

- **Warm up**
  - Each node accesses a pre-generated file test set (fixtures) that contains:
    - Small files (1KB to 100MB)
    - Medium files (100MB to 10GB)
    - Large files (10GB to 10TB)
    - Nested directories with multiple levels (10 to 100000) containing small files
- **Wave I** - IPFS Add with multiple layouts and sharding
  - Each node adds each file using Balanced Dag & Trickle Dag
  - Each node adds the nested directories using regular add and sharding
- **Wave II** - MFS
  - Each node calls files.write to create the Nested Directories structure in MFS. Once regular and once with Sharding
  - Each node runs a pin on the MFS root hash
  - Each node runs GC (to clean all the waste created by MFS)
- **Wave III** - URL Store & File Store
  - The Nested Directories get served beyond a HTTP static server
  - Each node adds (creates the manifest) for the nested directories using URL Store & File Store

### 2. Data Transfer of Random DataSets (Bitswap/GraphSync)

Create an environment in which data transfer is stress tested. This test is not about content discovery or connectivity, it is assumed that all nodes are dialable by each other and that these are executed in an homogeneous network (same CPU, Memory, Bandwidth).

#### What is being optimized (min/max, reach)

- (Minimize) The performance of fetching a file. Lower is Better
  - To compute this, capture: file size, number of nodes in the IPLD graph, time to fetch it from first block to completion.
- (Minimize) The bandwidth consumed to fetch a file. Lower is Better
  - To compute this, capture: The number of duplicated blocks received vs. total block count
- (Reach) The number of nodes that were able to fetch all files as instructed. (Reach 100% success of all fetches)
- (Reach) No node is expected to crash/panic during this Test Plan. (Reach 0% crashes)

#### Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - Ran with with an arbitraty amount of nodes (from 10 to 1000000) - N
- **Image Parameters**
  - Single Image - The go-ipfs commit that is being tested
    - Ran with custom libp2p & IPFS suites (swap in/out Bitswap & GraphSync versions, Crypto Channels, Transports and other libp2p components)
    - `File Sizes` - An array of File Sizes to be tested (default to: `[1MB, 1GB, 10GB, 100GB, 1TB]`)
    - `Directory Depth` - An Array containing objects that describe how deep/nested a directory goes and the size of files that can be found throughout (default to `[{depth: 10, size: 1MB}, {depth: 100, size: 1MB}]`

This test is not expected to support:

- An heterogeneus network in which nodes have different configurations

#### Narrative

- **Warm up**
  - Boot N nodes
  - Connect each node to the node next to it (hash ring)
  - Run multiple DHT random-walk queries to populate the finger tables
  - Run a discovery service provided by Redis (to ensure that every node keeps getting at least one another node to connect)
  - Each node creates a dataset with random data following the parameters `File Sizes` and `Directory Depth`
  - The nodes are divided in 4 cohorts, A, B, C & D, which each contains a set of %25 of the nodes available without creating an overlap (recommended to use a number of nodes that is a multiple of 4 to simplify the reasoning at the end (i.e. not having a situation in which a transfer of the file was instant))
- **Wave I**
  - Cohort B fetches the files created from Cohort A
- **Wave II**
  - Cohort C fetches the files created from Cohort A (expected to see speed improvements given that %50 of the network will have the file)
- **Wave III**
  - Cohort D fetches the files created from Cohort A (expected to see speed improvements given that %75 of the network will have the file)
- **Wave III**
  - Cohort D fetches the files created from Cohort A (expected to see speed improvements given that %75 of the network will have the file)
- **Wave IV**
  - Cohort A, B & C fetch the files created from Cohort D


### 3. Data Exchange with Datasets of Interest (BitSwap/GraphSync)

This test resembles the previous one (Data Transfer of Random DataSets (Bitswap/GraphSync)) with a twist. It focuses the attention to the experience will have when using IPFS to access popular and/or interesting datasets. The current datasets that this test plan contemplates are:

- Wikipedia Mirror
- NPM clone
- ImageNet

#### What is being optimized (min/max, reach)

- (Minimize) The performance of fetching a file. Lower is Better
  - To compute this, capture: file size, number of nodes in the IPLD graph, time to fetch it from first block to completion.
- (Minimize) The bandwidth consumed to fetch a file. Lower is Better
  - To compute this, capture: The number of duplicated blocks received vs. total block count
- (Reach) The number of nodes that were able to fetch all files as instructed. (Reach 100% success of all fetches)
- (Reach) No node is expected to crash/panic during this Test Plan. (Reach 0% crashes)


#### Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - Ran with with an arbitraty amount of nodes (from 10 to 1000000) - N
- **Image Parameters**
  - Number of nodes with full replica of the dataset initially (from 1 to 10) - M
  - Ran with custom libp2p & IPFS suites (swap in/out Bitswap & GraphSync versions, Crypto Channels, Transports and other libp2p components)

This test is not expected to support:

- An heterogeneus network in which nodes have different configurations

#### Narrative

- **Warm up**
  - Boot N nodes
  - Connect each node to the node next to it (hash ring)
  - Run multiple DHT random-walk queries to populate the finger tables
  - Run a discovery service provided by Redis (to ensure that every node keeps getting at least one another node to connect)
  - Load the datasets in M nodes
- **Wave I** - Access Wikipedia
  - %W of the nodes access Wikipedia
- **Wave II** - Access NPM
  - %W of the nodes access NPM
- **Wave III** - Access ImageNet
  - %W of the nodes access ImageNet

### 4. Nodes Connectivity (Transports, Hole Punching, Relay)

Ensuring that a node can always connect to the rest of the network, if not completely isolated.

#### What is being optimized (min/max, reach)

- (Reach) The number of nodes that are able to dial to any other node (100%)

#### Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - Ran with with an arbitraty amount of nodes (from 10 to 1000000) - N
  - Nodes being beyind a NAT/Firewall - F (F is a % of N)
  - Nodes running the Image with IPFS on a Browser - B (B is a % of N)
  - Nodes running the Image with go-ipfs using only TCP - T (T is a % of N)
  - Nodes running the Image with go-ipfs using only QUIC - Q (Q is a % of N)
  - Nodes running the Image with go-ipfs using only WebSockets - W (W is a % of N)
  - Nodes running the Image with go-ipfs using only WebRTC - C (C is a % of N)
- **Image Parameters**
  - Image A - Base `go-ipfs`
    - `Transport` The only transport to be used
  - Image B - Base `js-ipfs` running in a Browser
    - `Browser` The Browser in which js-ipfs will be running from

#### Narrative

- **Warm up**
  - Create the Bootstrapper nodes that are connected among themselves and support every transport
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -

### 5. Providing Content (Content Routing / DHT)

#### What is being optimized (min/max, reach)

#### Parameters

- **Network Parameters**
  -
- **Image Parameters**
  -

#### Narrative

- **Warm up**
  -
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -

### 6. Message Delivery (PubSub)

#### What is being optimized (min/max, reach)

#### Parameters

- **Network Parameters**
  -
- **Image Parameters**
  -

#### Narrative

- **Warm up**
  -
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -

### 7. Naming (IPNS & its multiple routers)

#### What is being optimized (min/max, reach)

#### Parameters

- **Network Parameters**
  -
- **Image Parameters**
  -


#### Narrative

- **Warm up**
  -
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -

### 8. ???

#### What is being optimized (min/max, reach)

#### Parameters

- **Network Parameters**
  -
- **Image Parameters**
  -


#### Narrative

- **Warm up**
  -
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -

### 9. ???

#### What is being optimized (min/max, reach)

#### Parameters

- **Network Parameters**
  -
- **Image Parameters**
  -


#### Narrative

- **Warm up**
  -
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -

### 10. ???

#### What is being optimized (min/max, reach)

#### Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  -
- **Image Parameters**
  -

#### Narrative

- **Warm up**
  -
- **Wave I**
  -
- **Wave II**
  -
- **Wave III**
  -