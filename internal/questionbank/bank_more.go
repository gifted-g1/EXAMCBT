package questionbank

// Gap is a fill-in-the-gap question. Each blank in Q is written as
// "______"; Blanks holds the accepted answers for each blank, in order.
// It is worth one mark per blank.
type Gap struct {
	Q      string
	Blanks [][]string
}

const b = "______"

// Subjects is the full bank: the core subjects plus the wider computing
// fields, each with 8 multiple-choice, 3 fill-in-the-gap and 2 written
// questions.
var Subjects = withGaps(append(append([]Subject{}, coreSubjects...), moreSubjects...))

func withGaps(ss []Subject) []Subject {
	for i := range ss {
		if g, ok := coreGaps[ss[i].Slug]; ok {
			ss[i].Gaps = g
		}
	}
	return ss
}

// Fill-in-the-gap questions for the core subjects (topics chosen so they
// do not repeat the multiple-choice questions).
var coreGaps = map[string][]Gap{
	"golang": {
		{"A Go type satisfies an interface " + b + ": it needs no implements declaration.", [][]string{{"implicitly", "automatically"}}},
		{"In Go, the zero value of an int is " + b + " and the zero value of a string is the " + b + " string.", [][]string{{"0", "zero"}, {"empty"}}},
		{"A Go module records its path and dependency versions in a file named " + b + ".", [][]string{{"go.mod"}}},
	},
	"python": {
		{"Python handles exceptions with a try block followed by one or more " + b + " blocks, and code in a " + b + " block always runs.", [][]string{{"except"}, {"finally"}}},
		{"The special method that initialises a new object of a Python class is named " + b + ".", [][]string{{"__init__"}}},
		{"An isolated per-project Python environment is created with the standard library module " + b + ".", [][]string{{"venv"}}},
	},
	"javascript": {
		{"In the browser, the tree of objects that represents a web page is the " + b + ", and an element can be found by its id with document." + b + "(id).", [][]string{{"DOM", "Document Object Model"}, {"getElementById"}}},
		{"The runtime that executes JavaScript on servers, outside the browser, is " + b + ".", [][]string{{"Node.js", "Node", "NodeJS"}}},
		{"A Promise that completes successfully is " + b + ", and one that fails is " + b + ".", [][]string{{"fulfilled", "resolved"}, {"rejected"}}},
	},
	"php": {
		{"In PHP, the superglobal that holds URL query-string parameters is " + b + ".", [][]string{{"$_GET"}}},
		{"The PHP function that converts an array into a JSON string is " + b + ".", [][]string{{"json_encode", "json_encode()"}}},
		{"To check a login password against a hash made by password_hash(), PHP code calls " + b + ".", [][]string{{"password_verify", "password_verify()"}}},
	},
	"laravel": {
		{"Laravel validation rules can be kept in a dedicated Form " + b + " class instead of the controller.", [][]string{{"Request"}}},
		{"Laravel reads environment-specific settings, such as the database password, from a file named " + b + ".", [][]string{{".env"}}},
		{"Slow work such as sending email can be pushed to background workers with Laravel " + b + ".", [][]string{{"queues", "queue", "jobs"}}},
	},
	"computer-science": {
		{"Breadth-first search uses a " + b + " to hold the nodes to visit, while depth-first search uses a " + b + " or recursion.", [][]string{{"queue"}, {"stack"}}},
		{"A tree in which every node has at most two children is called a " + b + " tree.", [][]string{{"binary"}}},
		{"A problem is NP-complete if it is in NP and every problem in NP can be " + b + " to it in polynomial time.", [][]string{{"reduced"}}},
	},
	"information-technology": {
		{"The command that shows the route packets take to a destination is " + b + " on Linux and tracert on Windows.", [][]string{{"traceroute"}}},
		{"A device that keeps computers running for a short time during a power failure is a(n) " + b + ".", [][]string{{"UPS", "uninterruptible power supply"}}},
		{"The widely used framework of IT service management practices, such as incident and change management, is " + b + ".", [][]string{{"ITIL"}}},
	},
	"information-systems": {
		{"A diagram showing how data moves between processes, data stores and external entities is a " + b + ".", [][]string{{"data flow diagram", "DFD"}}},
		{"A system that gives senior executives summarised, high-level information is an " + b + " system.", [][]string{{"executive information", "EIS", "executive support", "ESS"}}},
		{"Business " + b + " tools analyse historical data to produce dashboards and reports for decision makers.", [][]string{{"intelligence"}}},
	},
	"cyber-security": {
		{"A " + b + " filters network traffic according to a set of security rules.", [][]string{{"firewall"}}},
		{"Malware that encrypts a victim's files and demands payment for the key is called " + b + ".", [][]string{{"ransomware"}}},
		{"Simulating a real attack on a system, with the owner's permission, to find weaknesses is called penetration " + b + ".", [][]string{{"testing"}}},
	},
	"software-engineering": {
		{"A Git " + b + " lets a developer work on a feature in isolation before merging it.", [][]string{{"branch"}}},
		{"The design pattern that ensures a class has only one instance is the " + b + " pattern.", [][]string{{"Singleton"}}},
		{"Re-running tests to check that existing features still work after a change is " + b + " testing.", [][]string{{"regression"}}},
	},
	"devops-engineering": {
		{"Releasing a new version to a small share of users before everyone else is a " + b + " deployment.", [][]string{{"canary"}}},
		{"Ansible, Puppet and Chef are " + b + " management tools.", [][]string{{"configuration"}}},
		{"Keeping the desired state of infrastructure in Git and applying changes automatically from it is called " + b + ".", [][]string{{"GitOps"}}},
	},
}

// moreSubjects widen the bank to the rest of computing.
var moreSubjects = []Subject{
	{
		Name: "Computer Networks", Slug: "computer-networks",
		Blurb: "Layers, addressing, routing and core Internet protocols.",
		MCQs: []MCQ{
			{"Which OSI layer is responsible for routing packets between networks?", []string{"Network", "Data link", "Transport", "Session"}, 0},
			{"Which protocol finds the MAC address that belongs to an IPv4 address on a local network?", []string{"ARP", "DNS", "ICMP", "NAT"}, 0},
			{"How many usable host addresses does a /24 IPv4 subnet provide?", []string{"254", "256", "255", "252"}, 0},
			{"Which transport protocol provides reliable, ordered delivery of data?", []string{"TCP", "UDP", "IP", "ICMP"}, 0},
			{"Which protocol transfers email between mail servers?", []string{"SMTP", "IMAP", "POP3", "SNMP"}, 0},
			{"Which of these is a link-state routing protocol?", []string{"OSPF", "RIP", "BGP", "RIPng"}, 0},
			{"Which device forwards frames inside a LAN using MAC addresses?", []string{"Switch", "Router", "Hub", "Repeater"}, 0},
			{"What does NAT mainly allow?", []string{"Many private hosts to share one public IP address", "Encryption of all traffic", "Automatic IP address assignment", "Translation of names into addresses"}, 0},
		},
		Gaps: []Gap{
			{"The ping utility sends messages using the " + b + " protocol.", [][]string{{"ICMP"}}},
			{"DNS queries normally use port " + b + ".", [][]string{{"53"}}},
			{"An IPv6 address is " + b + " bits long, compared with " + b + " bits for IPv4.", [][]string{{"128"}, {"32"}}},
		},
		Written: []Written{
			{"Explain the TCP three-way handshake and why a connection needs it.", 5},
			{"Compare IPv4 and IPv6 in terms of address size, header design and the need for NAT.", 5},
		},
	},
	{
		Name: "Internet of Things and Embedded Systems", Slug: "iot-embedded",
		Blurb: "Sensors, microcontrollers, IoT protocols and edge computing.",
		MCQs: []MCQ{
			{"Which lightweight publish/subscribe protocol is widely used in IoT?", []string{"MQTT", "FTP", "SMTP", "SNMP"}, 0},
			{"In MQTT, which component receives messages from publishers and passes them to subscribers?", []string{"Broker", "Gateway", "Router", "Client"}, 0},
			{"Which wireless technology is designed for low-power IoT links over several kilometres?", []string{"LoRaWAN", "Bluetooth Classic", "Wi-Fi 6", "NFC"}, 0},
			{"A microcontroller turns an analog sensor voltage into a number using a(n):", []string{"ADC", "DAC", "PWM output", "UART"}, 0},
			{"Which technique varies the duty cycle of a signal to control motor speed or LED brightness?", []string{"PWM", "I2C", "SPI", "ADC"}, 0},
			{"Which two-wire serial bus uses SDA and SCL lines to connect several devices?", []string{"I2C", "SPI", "UART", "Ethernet"}, 0},
			{"An embedded designer chooses a real-time operating system (RTOS) mainly for:", []string{"Deterministic task scheduling and timing", "A graphical desktop", "Unlimited memory", "Built-in cloud storage"}, 0},
			{"Why is CoAP suited to constrained IoT devices?", []string{"It runs over UDP with a compact binary header", "It needs a persistent TCP connection", "It only uses XML messages", "It requires a web browser"}, 0},
		},
		Gaps: []Gap{
			{"Processing data close to where it is produced, instead of in a distant data centre, is called " + b + " computing.", [][]string{{"edge"}}},
			{"MQTT messages are published to named " + b + ", and the highest MQTT QoS level is " + b + ".", [][]string{{"topics", "topic"}, {"2", "two"}}},
			{"A " + b + " timer automatically resets a microcontroller if its software stops responding.", [][]string{{"watchdog"}}},
		},
		Written: []Written{
			{"Describe a layered IoT architecture (devices, gateway, cloud, application) for a smart-farming system, and explain where data should be processed and why.", 5},
			{"Discuss three security challenges specific to IoT devices and one mitigation for each.", 5},
		},
	},
	{
		Name: "Artificial Intelligence and Machine Learning", Slug: "ai-ml",
		Blurb: "Learning paradigms, model evaluation, neural networks and responsible AI.",
		MCQs: []MCQ{
			{"Training a model on labelled input-output examples is called:", []string{"Supervised learning", "Unsupervised learning", "Reinforcement learning", "Clustering"}, 0},
			{"Which of these is an unsupervised clustering algorithm?", []string{"k-means", "Linear regression", "Logistic regression", "Naive Bayes"}, 0},
			{"Which metric is the harmonic mean of precision and recall?", []string{"F1 score", "Accuracy", "ROC AUC", "Mean squared error"}, 0},
			{"Which technique reduces overfitting by randomly switching off neurons during training?", []string{"Dropout", "Momentum", "Padding", "Pooling"}, 0},
			{"Which neural network architecture underlies modern large language models?", []string{"Transformer", "Convolutional neural network", "Single perceptron", "Hopfield network"}, 0},
			{"In reinforcement learning, what does an agent try to maximise?", []string{"Cumulative reward", "Training loss", "Number of labels", "Learning rate"}, 0},
			{"Splitting data into k parts and training k times, holding out a different part each time, is called:", []string{"k-fold cross-validation", "Bootstrapping", "Bagging", "Grid search"}, 0},
			{"Which activation function outputs max(0, x)?", []string{"ReLU", "Sigmoid", "Tanh", "Softmax"}, 0},
		},
		Gaps: []Gap{
			{"Neural networks compute gradients by applying the chain rule backwards through the layers, an algorithm called " + b + ".", [][]string{{"backpropagation", "back propagation", "back-propagation"}}},
			{"Data used to tune hyperparameters is the " + b + " set, and data kept aside for the final estimate of performance is the " + b + " set.", [][]string{{"validation"}, {"test"}}},
			{"A " + b + " matrix tabulates true positives, false positives, true negatives and false negatives.", [][]string{{"confusion"}}},
		},
		Written: []Written{
			{"Explain the bias-variance trade-off and how it relates to underfitting and overfitting.", 5},
			{"Discuss two sources of unfair bias in machine-learning systems and how responsible-AI practice reduces them.", 5},
		},
	},
	{
		Name: "Data Science and Databases", Slug: "data-databases",
		Blurb: "SQL, database design, transactions, warehousing and analytics.",
		MCQs: []MCQ{
			{"Which SQL command removes rows from a table?", []string{"DELETE", "DROP", "REMOVE", "ERASE"}, 0},
			{"Which SQL keyword removes duplicate rows from a query result?", []string{"DISTINCT", "UNIQUE ROWS", "SINGLE", "ONLY"}, 0},
			{"Which normal form removes transitive dependencies?", []string{"Third normal form", "First normal form", "Second normal form", "Unnormalised form"}, 0},
			{"Which join returns every row of the left table plus matching rows from the right table?", []string{"LEFT JOIN", "INNER JOIN", "CROSS JOIN", "SELF JOIN"}, 0},
			{"Which kind of database stores records as flexible JSON-like documents?", []string{"Document database", "Relational database", "Spreadsheet", "Flat file"}, 0},
			{"In data engineering, ETL stands for:", []string{"Extract, Transform, Load", "Encrypt, Transfer, Log", "Evaluate, Test, Launch", "Export, Train, Learn"}, 0},
			{"Which measure of central tendency is least affected by outliers?", []string{"Median", "Mean", "Range", "Variance"}, 0},
			{"An index on a column mainly speeds up:", []string{"Searching and filtering on that column", "Inserting new rows", "Taking backups", "Changing the schema"}, 0},
		},
		Gaps: []Gap{
			{"In SQL, rows that share a value are combined with GROUP " + b + ", and the resulting groups are filtered with " + b + ".", [][]string{{"BY"}, {"HAVING"}}},
			{"A foreign key enforces " + b + " integrity between two tables.", [][]string{{"referential"}}},
			{"In a data-warehouse star schema, a central " + b + " table is surrounded by dimension tables.", [][]string{{"fact"}}},
		},
		Written: []Written{
			{"Explain the ACID properties of a transaction, with an example of what goes wrong without each.", 5},
			{"Describe the stages of a data-science project from problem definition to deployment, and the role of data cleaning.", 5},
		},
	},
	{
		Name: "Cloud Computing", Slug: "cloud-computing",
		Blurb: "Service and deployment models, scaling, storage and shared responsibility.",
		MCQs: []MCQ{
			{"Which cloud service model gives customers virtual machines, storage and networks to manage themselves?", []string{"IaaS", "PaaS", "SaaS", "FaaS"}, 0},
			{"Gmail and Microsoft 365 are examples of which service model?", []string{"SaaS", "IaaS", "PaaS", "On-premises software"}, 0},
			{"Serverless functions are mainly billed by:", []string{"Invocations and execution time", "Servers rented per month", "A fixed annual licence", "Disk size only"}, 0},
			{"Automatically adding instances when load rises is called:", []string{"Auto-scaling", "Vertical scaling", "Load shedding", "Caching"}, 0},
			{"In IaaS, who secures the physical data centre under the shared responsibility model?", []string{"The cloud provider", "The customer", "Both equally", "Nobody"}, 0},
			{"Which tool orchestrates containers across a cluster of machines?", []string{"Kubernetes", "Git", "Jenkins", "Grafana"}, 0},
			{"A deployment combining a private data centre with a public cloud is a:", []string{"Hybrid cloud", "Community cloud", "Single-tenant cloud", "Edge cloud"}, 0},
			{"Object storage services such as Amazon S3 store data as:", []string{"Objects in buckets", "Blocks on a mounted disk", "Rows in tables", "Files on tape"}, 0},
		},
		Gaps: []Gap{
			{"Paying only for the resources actually consumed is called " + b + "-as-you-go pricing.", [][]string{{"pay"}}},
			{"Incoming traffic is spread across several servers by a load " + b + ".", [][]string{{"balancer"}}},
			{"Running code without provisioning or managing servers is known as " + b + " computing.", [][]string{{"serverless"}}},
		},
		Written: []Written{
			{"Compare IaaS, PaaS and SaaS, giving an example of each and the customer's responsibilities in each.", 5},
			{"Discuss vendor lock-in and data sovereignty risks when a university moves to the cloud, and how to reduce them.", 5},
		},
	},
	{
		Name: "Computer Architecture and Operating Systems", Slug: "architecture-os",
		Blurb: "Processors, memory hierarchy, processes, scheduling and concurrency.",
		MCQs: []MCQ{
			{"Which CPU component performs arithmetic and logical operations?", []string{"ALU", "Control unit", "Program counter", "Cache"}, 0},
			{"Which of these memories is fastest?", []string{"CPU registers", "L3 cache", "Main memory (RAM)", "Solid-state drive"}, 0},
			{"Deadlock needs mutual exclusion, hold and wait, no preemption and:", []string{"Circular wait", "Starvation", "Paging", "Thrashing"}, 0},
			{"Which scheduling algorithm can starve long jobs?", []string{"Shortest job first", "Round robin", "First-come, first-served", "Fair-share scheduling"}, 0},
			{"Virtual memory lets a process:", []string{"Use more memory than is physically installed", "Run without a CPU", "Bypass the operating system", "Share CPU registers with other processes"}, 0},
			{"Code running in kernel mode differs from user mode because it:", []string{"Has unrestricted access to the hardware", "Can only run applications", "Cannot handle interrupts", "Has no memory of its own"}, 0},
			{"With byte addressing, a 32-bit address can reach at most:", []string{"4 GB", "2 GB", "16 GB", "64 KB"}, 0},
			{"Which number system uses base 16?", []string{"Hexadecimal", "Binary", "Octal", "Decimal"}, 0},
		},
		Gaps: []Gap{
			{"The hexadecimal number FF equals " + b + " in decimal.", [][]string{{"255"}}},
			{"When the CPU stops one process and starts another, the operating system performs a context " + b + ".", [][]string{{"switch"}}},
			{"Excessive paging that keeps the CPU waiting for the disk is called " + b + ".", [][]string{{"thrashing"}}},
		},
		Written: []Written{
			{"Explain the fetch-decode-execute cycle and how pipelining improves throughput.", 5},
			{"Compare processes and threads, and explain how a race condition occurs and how it can be prevented.", 5},
		},
	},
	{
		Name: "Distributed Systems and Blockchain", Slug: "distributed-blockchain",
		Blurb: "Consistency, consensus, replication, fault tolerance and blockchains.",
		MCQs: []MCQ{
			{"By the CAP theorem, during a network partition a distributed system must choose between:", []string{"Consistency and availability", "Speed and cost", "Security and usability", "Latency and storage"}, 0},
			{"Which algorithm lets replicated servers agree on a shared log?", []string{"Raft", "Dijkstra's algorithm", "RSA", "Huffman coding"}, 0},
			{"Bitcoin miners compete to add blocks using:", []string{"Proof of work", "Proof of stake", "Round-robin turns", "Two-phase commit"}, 0},
			{"A smart contract is:", []string{"Code stored and executed on a blockchain", "A legal document signed online", "A cloud service-level agreement", "An encrypted email"}, 0},
			{"Replicating data across several nodes mainly improves:", []string{"Availability and fault tolerance", "Code readability", "Compile speed", "Screen resolution"}, 0},
			{"Two-phase commit is used to:", []string{"Make a transaction atomic across several nodes", "Collect garbage memory", "Route network packets", "Hash passwords"}, 0},
			{"Eventual consistency means that:", []string{"All replicas converge if no new updates arrive", "Every read returns the latest write", "Data is never replicated", "Concurrent writes are rejected"}, 0},
			{"What makes past blocks in a blockchain tamper-evident?", []string{"Each block contains the hash of the previous block", "Blocks are kept in RAM", "Blocks are compressed", "Blocks are numbered in order"}, 0},
		},
		Gaps: []Gap{
			{"A system that keeps working correctly when some of its components fail is fault " + b + ".", [][]string{{"tolerant"}}},
			{"In Raft, one server is elected " + b + " and the others act as followers.", [][]string{{"leader"}}},
			{"Splitting a large database across machines by key ranges or hashes is called " + b + ".", [][]string{{"sharding", "partitioning", "horizontal partitioning"}}},
		},
		Written: []Written{
			{"Explain why clocks cannot be perfectly synchronised in a distributed system, and how logical clocks order events.", 5},
			{"Evaluate whether a blockchain suits storing university certificates, compared with a conventional database.", 5},
		},
	},
	{
		Name: "Human-Computer Interaction and Mobile Development", Slug: "hci-mobile",
		Blurb: "Usability, accessibility, interaction design and mobile app platforms.",
		MCQs: []MCQ{
			{"Which usability heuristic says users should always know what the system is doing?", []string{"Visibility of system status", "Aesthetic and minimalist design", "Error prevention", "Flexibility and efficiency of use"}, 0},
			{"The WCAG guidelines are concerned with:", []string{"Web accessibility", "Database tuning", "Network routing", "Encryption strength"}, 0},
			{"A low-fidelity sketch of a screen's layout is called a:", []string{"Wireframe", "Compiler", "Sprint", "Schema"}, 0},
			{"Which language does Google recommend for modern Android apps?", []string{"Kotlin", "Swift", "Objective-C", "Perl"}, 0},
			{"Which framework builds iOS and Android apps from one Dart codebase?", []string{"Flutter", "React", "Django", "Laravel"}, 0},
			{"Usability testing mainly measures:", []string{"How easily real users complete tasks", "Server uptime", "Code coverage", "Battery chemistry"}, 0},
			{"Responsive web design adapts a page's layout using:", []string{"CSS media queries", "Database triggers", "Cron jobs", "Firewall rules"}, 0},
			{"Fitts's law says the time to reach a target depends on:", []string{"Its distance and size", "Its colour", "Screen brightness", "Network speed"}, 0},
		},
		Gaps: []Gap{
			{"Screen readers announce images using the text in the HTML " + b + " attribute.", [][]string{{"alt"}}},
			{"A clickable mock-up used to test a design with users before it is built is a " + b + ".", [][]string{{"prototype"}}},
			{"Apple's main programming language for iOS apps is " + b + ".", [][]string{{"Swift"}}},
		},
		Written: []Written{
			{"Describe a user-centred design process for a student exam app, including how users are involved at each stage.", 5},
			{"Compare native and cross-platform mobile development in terms of performance, cost and access to device features.", 5},
		},
	},
}
