// Package questionbank holds Exam Shield's sample question bank (11 subjects)
// and renders it as PDFs in the exact format the PDF importer understands.
package questionbank

// MCQ is a multiple-choice question. Answer indexes Options as written here;
// the generator shuffles option order deterministically.
type MCQ struct {
	Q       string
	Options []string
	Answer  int
}

// Written is a free-text question.
type Written struct {
	Q     string
	Marks float64
}

// Subject is one section of the bank.
type Subject struct {
	Name    string
	Slug    string
	Blurb   string
	MCQs    []MCQ
	Written []Written
}

// Subjects is the full bank: 8 multiple-choice + 2 written questions each.
var Subjects = []Subject{
	{
		Name: "Golang", Slug: "golang",
		Blurb: "Concurrency, error handling, types and tooling in Go.",
		MCQs: []MCQ{
			{"Which keyword starts a new goroutine?", []string{"go", "async", "spawn", "thread"}, 0},
			{"What is the value of a map declared with var m map[string]int before it is initialised?", []string{"nil", "An empty map ready for writes", "A map with one zero entry", "It does not compile"}, 0},
			{"What happens when a program writes to a nil map in Go?", []string{"A runtime panic", "The map is created automatically", "A compile-time error", "The write is silently ignored"}, 0},
			{"Which statement schedules a function call to run when the surrounding function returns?", []string{"defer", "finally", "later", "await"}, 0},
			{"How does idiomatic Go report a recoverable failure from a function?", []string{"By returning an error value as the last result", "By throwing an exception", "By setting a global errno variable", "By calling panic in every case"}, 0},
			{"Which built-in function creates a channel of integers?", []string{"make(chan int)", "new(chan int)", "chan.New(int)", "channel(int)"}, 0},
			{"What does a select statement do in Go?", []string{"Waits on several channel operations and runs the one that is ready", "Selects rows from a database table", "Sorts the elements of a slice", "Chooses which goroutine the scheduler stops"}, 0},
			{"Which command formats Go source code to the standard style?", []string{"gofmt", "golint", "go style", "gopretty"}, 0},
		},
		Written: []Written{
			{"Explain the difference between an array and a slice in Go, including how each is passed to a function.", 4},
			{"What is a data race? Describe how the Go race detector helps you find one.", 4},
		},
	},
	{
		Name: "Python", Slug: "python",
		Blurb: "Core syntax, data types and idioms of Python 3.",
		MCQs: []MCQ{
			{"Which of these Python types is immutable?", []string{"tuple", "list", "dict", "set"}, 0},
			{"Which keyword defines a function in Python?", []string{"def", "func", "function", "fn"}, 0},
			{"What does len(\"hello\") return?", []string{"5", "4", "6", "An error"}, 0},
			{"What is the result of the expression 7 // 2 in Python 3?", []string{"3", "3.5", "4", "1"}, 0},
			{"Which line is a valid dictionary comprehension?", []string{"{k: v for k, v in pairs}", "[k: v for k, v in pairs]", "(k: v for k, v in pairs)", "dict[k for k in pairs]"}, 0},
			{"What does a with open(path) as fh: block guarantee?", []string{"The file is closed when the block exits", "The file is opened read-only", "The file is locked for other processes", "The file is deleted afterwards"}, 0},
			{"Which tool installs third-party Python packages?", []string{"pip", "npm", "gem", "composer"}, 0},
			{"What does print(type([])) display?", []string{"<class 'list'>", "<class 'tuple'>", "<class 'array'>", "<class 'dict'>"}, 0},
		},
		Written: []Written{
			{"Explain the Global Interpreter Lock (GIL) and its effect on multi-threaded, CPU-bound Python code.", 4},
			{"Compare a list with a generator. When is a generator the better choice?", 4},
		},
	},
	{
		Name: "JavaScript", Slug: "javascript",
		Blurb: "Language fundamentals, types and asynchronous programming.",
		MCQs: []MCQ{
			{"Which keyword declares a block-scoped variable that cannot be reassigned?", []string{"const", "var", "let", "static"}, 0},
			{"What does typeof null evaluate to?", []string{"\"object\"", "\"null\"", "\"undefined\"", "\"number\""}, 0},
			{"What is the result of \"5\" + 3 in JavaScript?", []string{"\"53\"", "8", "NaN", "A TypeError"}, 0},
			{"Which operator compares both value and type without coercion?", []string{"===", "==", "=", "=>"}, 0},
			{"Which method turns a JSON string into a JavaScript object?", []string{"JSON.parse", "JSON.stringify", "JSON.toObject", "Object.fromJSON"}, 0},
			{"Which syntax, standardised in ES2017, lets you write promise-based code that reads synchronously?", []string{"async / await", "Callbacks", "Generators with yield", "XMLHttpRequest"}, 0},
			{"Which array method returns a new array containing each element transformed by a function?", []string{"map", "forEach", "push", "splice"}, 0},
			{"When does a setTimeout callback with a delay of 0 ms run?", []string{"After the current call stack and pending microtasks finish", "Immediately, before the next line of code", "Before any pending promise callbacks", "Never; a delay of 0 is ignored"}, 0},
		},
		Written: []Written{
			{"Explain what a closure is and give one practical use case.", 4},
			{"Describe the differences between var, let and const.", 3},
		},
	},
	{
		Name: "PHP", Slug: "php",
		Blurb: "Server-side PHP: syntax, forms, security and tooling.",
		MCQs: []MCQ{
			{"Which character begins every variable name in PHP?", []string{"$", "@", "#", "&"}, 0},
			{"Which superglobal holds data submitted by an HTML form using the POST method?", []string{"$_POST", "$_GET", "$_FORM", "$_SERVER"}, 0},
			{"Which operator joins two strings in PHP?", []string{". (dot)", "+ (plus)", "& (ampersand)", "++ (increment)"}, 0},
			{"Which function checks that a variable exists and is not null?", []string{"isset()", "empty()", "is_null()", "defined()"}, 0},
			{"What is the correct way to prevent SQL injection when using PDO?", []string{"Prepared statements with bound parameters", "Calling addslashes() on input", "Hashing the input with md5()", "Running strip_tags() on input"}, 0},
			{"Which tool manages PHP project dependencies?", []string{"Composer", "npm", "pip", "Maven"}, 0},
			{"Which function should be used to hash user passwords?", []string{"password_hash()", "md5()", "sha1()", "base64_encode()"}, 0},
			{"What does echo 10 <=> 5; print?", []string{"1", "-1", "0", "true"}, 0},
		},
		Written: []Written{
			{"Explain the difference between include and require in PHP.", 3},
			{"What are PHP sessions? List two measures that keep session data secure.", 4},
		},
	},
	{
		Name: "Laravel", Slug: "laravel",
		Blurb: "The Laravel framework: routing, Eloquent, Blade and Artisan.",
		MCQs: []MCQ{
			{"What is the name of Laravel's command-line tool?", []string{"Artisan", "Blade", "Forge", "Tinker"}, 0},
			{"Which templating engine ships with Laravel?", []string{"Blade", "Twig", "Smarty", "Mustache"}, 0},
			{"What is Laravel's built-in ORM called?", []string{"Eloquent", "Doctrine", "Hibernate", "Sequelize"}, 0},
			{"Which command creates a new migration file?", []string{"php artisan make:migration create_posts_table", "php artisan migrate:new posts", "composer make:migration posts", "php artisan db:create posts"}, 0},
			{"In which file are browser (web) routes normally defined?", []string{"routes/web.php", "app/routes.php", "config/routes.php", "public/routes.php"}, 0},
			{"What is the main purpose of middleware in Laravel?", []string{"Filtering HTTP requests entering the application", "Rendering Blade views", "Defining the database schema", "Compiling front-end assets"}, 0},
			{"Which Blade directive adds a CSRF token field to a form?", []string{"@csrf", "@token", "@secure", "@protect"}, 0},
			{"Which command applies all pending database migrations?", []string{"php artisan migrate", "php artisan db:run", "composer migrate", "php artisan schema:apply"}, 0},
		},
		Written: []Written{
			{"Explain mass assignment and how the $fillable property protects against it.", 4},
			{"Describe Laravel's service container and how it supports dependency injection.", 4},
		},
	},
	{
		Name: "Computer Science", Slug: "computer-science",
		Blurb: "Algorithms, data structures, number systems and systems.",
		MCQs: []MCQ{
			{"What is the time complexity of binary search on a sorted array?", []string{"O(log n)", "O(n)", "O(n log n)", "O(1)"}, 0},
			{"Which data structure follows Last-In, First-Out (LIFO) order?", []string{"Stack", "Queue", "Linked list", "Binary search tree"}, 0},
			{"What is the average-case time complexity of quicksort?", []string{"O(n log n)", "O(n^2)", "O(log n)", "O(n)"}, 0},
			{"What is the binary representation of decimal 10?", []string{"1010", "1001", "1100", "0110"}, 0},
			{"Which algorithm finds shortest paths in a graph with non-negative edge weights?", []string{"Dijkstra's algorithm", "Depth-first search", "Kruskal's algorithm", "Prim's algorithm"}, 0},
			{"What does a compiler do?", []string{"Translates source code into machine or target code", "Translates machine code back into source code", "Executes source code one line at a time", "Compresses files for storage"}, 0},
			{"What is the average time to look up a key in a hash table?", []string{"O(1)", "O(log n)", "O(n)", "O(n^2)"}, 0},
			{"Which of these is NOT a core operating system function?", []string{"Compiling user programs", "Process scheduling", "Memory management", "File management"}, 0},
		},
		Written: []Written{
			{"Explain recursion and the role of the base case. Give a simple example.", 3},
			{"Compare a stack and a queue, giving one real-world use for each.", 4},
		},
	},
	{
		Name: "Information Technology", Slug: "information-technology",
		Blurb: "Networking, infrastructure, cloud services and IT operations.",
		MCQs: []MCQ{
			{"Which protocol automatically assigns IP addresses to devices on a network?", []string{"DHCP", "DNS", "FTP", "SMTP"}, 0},
			{"What is the main job of DNS?", []string{"Translating domain names into IP addresses", "Encrypting web traffic", "Assigning MAC addresses", "Routing email between servers"}, 0},
			{"What is the default port for HTTPS?", []string{"443", "80", "21", "8080"}, 0},
			{"Which device forwards traffic between different networks?", []string{"Router", "Hub", "Switch", "Repeater"}, 0},
			{"What does RAID 1 provide?", []string{"Mirroring of data across disks", "Striping without redundancy", "Distributed parity across three disks", "Data compression"}, 0},
			{"Which of these is a cloud service model?", []string{"SaaS (Software as a Service)", "LAN (Local Area Network)", "VPN (Virtual Private Network)", "DHCP (Dynamic Host Configuration Protocol)"}, 0},
			{"How many bits are in an IPv4 address?", []string{"32", "16", "64", "128"}, 0},
			{"What does the 3-2-1 backup rule recommend?", []string{"3 copies of data, on 2 types of media, with 1 copy offsite", "3 servers, 2 administrators and 1 user", "Daily, weekly and monthly backups only", "3 disks in a RAID 2 array plus 1 spare"}, 0},
		},
		Written: []Written{
			{"Explain the difference between a LAN and a WAN, with one example of each.", 3},
			{"Describe the role of an IT help desk and how a ticketing system supports it.", 4},
		},
	},
	{
		Name: "Information Systems", Slug: "information-systems",
		Blurb: "How organisations use data, systems and processes.",
		MCQs: []MCQ{
			{"What does TPS stand for in information systems?", []string{"Transaction Processing System", "Total Planning Service", "Text Processing Software", "Team Performance System"}, 0},
			{"What does an ERP system do?", []string{"Integrates core business processes across departments", "Handles only accounting", "Hosts the company website", "Manages only email"}, 0},
			{"How does information differ from data?", []string{"Information is data processed into a meaningful form", "Information is raw, unprocessed facts", "Information refers to computer hardware", "There is no difference"}, 0},
			{"What is a Decision Support System (DSS) mainly used for?", []string{"Helping managers analyse options and make decisions", "Printing documents", "Routing network traffic", "Running payroll only"}, 0},
			{"What is the main goal of database normalisation?", []string{"Reducing data redundancy", "Increasing hardware speed", "Adding more users", "Encrypting the database"}, 0},
			{"Which SDLC phase identifies what users need from a new system?", []string{"Requirements analysis", "Testing", "Deployment", "Maintenance"}, 0},
			{"What does a CRM system focus on?", []string{"Managing relationships with customers", "Managing source code", "Managing CPU scheduling", "Managing legal compliance"}, 0},
			{"What does a primary key do in a relational table?", []string{"Uniquely identifies each row", "Encrypts the table", "Sorts rows alphabetically", "Links to a table in another database only"}, 0},
		},
		Written: []Written{
			{"Explain how a Management Information System (MIS) supports middle management.", 4},
			{"Discuss two ethical issues that arise from the use of information systems.", 4},
		},
	},
	{
		Name: "Cyber Security", Slug: "cyber-security",
		Blurb: "Security principles, threats, cryptography and defence.",
		MCQs: []MCQ{
			{"What do the letters of the CIA triad stand for?", []string{"Confidentiality, Integrity, Availability", "Control, Inspection, Audit", "Cipher, Identity, Access", "Compliance, Integrity, Authentication"}, 0},
			{"What is phishing?", []string{"Tricking people into revealing information through fake messages", "Capturing packets on a network", "Flooding a server with requests", "Guessing passwords by trying every combination"}, 0},
			{"Which of these is an asymmetric (public-key) algorithm?", []string{"RSA", "AES", "DES", "3DES"}, 0},
			{"What is the most effective defence against SQL injection?", []string{"Parameterised queries", "Longer passwords", "Using HTTPS", "Installing a firewall"}, 0},
			{"What does multi-factor authentication require?", []string{"Two or more independent proofs of identity", "Two or more firewalls", "Two or more administrator accounts", "Two or more backups"}, 0},
			{"Which property describes a cryptographic hash function?", []string{"It is one-way and cannot practically be reversed", "It can be decrypted with the right key", "It compresses files losslessly", "It is the same as Base64 encoding"}, 0},
			{"A DDoS attack mainly threatens which security goal?", []string{"Availability", "Confidentiality", "Integrity", "Non-repudiation"}, 0},
			{"What does the principle of least privilege mean?", []string{"Users get only the access they need to do their work", "Every user is an administrator", "Passwords are not required", "All users have read-only access to everything"}, 0},
		},
		Written: []Written{
			{"Explain the difference between symmetric and asymmetric encryption, with an example of each.", 4},
			{"Describe the main phases of an incident response plan.", 5},
		},
	},
	{
		Name: "Software Engineering", Slug: "software-engineering",
		Blurb: "Processes, design principles, testing and collaboration.",
		MCQs: []MCQ{
			{"Which agile framework organises work into fixed-length sprints?", []string{"Scrum", "Waterfall", "V-model", "Spiral"}, 0},
			{"What does unit testing focus on?", []string{"The smallest testable parts of a program", "The whole system end to end", "Only the user interface", "Only the hardware"}, 0},
			{"Which Git command records staged changes in the repository history?", []string{"git commit", "git push", "git clone", "git status"}, 0},
			{"In the SOLID principles, what does the S stand for?", []string{"Single responsibility", "Simple design", "Static typing", "Secure coding"}, 0},
			{"What is refactoring?", []string{"Improving code structure without changing its behaviour", "Adding new features", "Deleting failing tests", "Rewriting the program in another language"}, 0},
			{"What does CI stand for in modern development practice?", []string{"Continuous Integration", "Code Inspection", "Central Index", "Compiled Interface"}, 0},
			{"Which UML diagram shows how objects interact over time?", []string{"Sequence diagram", "Class diagram", "Deployment diagram", "Entity-relationship diagram"}, 0},
			{"What is technical debt?", []string{"The future cost of shortcuts taken in code today", "Money owed for software licences", "Loans used to buy hardware", "Unpaid developer salaries"}, 0},
		},
		Written: []Written{
			{"Compare the Waterfall and Agile approaches to software development.", 5},
			{"Explain the purpose of code reviews and two benefits they bring to a team.", 3},
		},
	},
	{
		Name: "DevOps Engineering", Slug: "devops-engineering",
		Blurb: "Containers, orchestration, automation and observability.",
		MCQs: []MCQ{
			{"In Docker, what is a container?", []string{"A running instance of an image", "A script that builds an image", "A registry that stores images", "A hypervisor for virtual machines"}, 0},
			{"Which file describes how to build a Docker image?", []string{"Dockerfile", "Makefile", "Jenkinsfile", "package.json"}, 0},
			{"What is the smallest deployable unit in Kubernetes?", []string{"Pod", "Node", "Cluster", "Namespace"}, 0},
			{"Which tool is used for Infrastructure as Code?", []string{"Terraform", "Wireshark", "Photoshop", "Postman"}, 0},
			{"What is a blue-green deployment?", []string{"Running two identical environments and switching traffic between them", "A colour scheme for dashboards", "A code formatting standard", "A debugging technique"}, 0},
			{"Which tool collects and stores time-series metrics for monitoring?", []string{"Prometheus", "Babel", "Figma", "Webpack"}, 0},
			{"In a CI/CD pipeline, what normally happens in the test stage?", []string{"Automated tests run against the new build", "The release is announced to customers", "Old builds are archived", "Servers are physically installed"}, 0},
			{"Which command lists the running Docker containers?", []string{"docker ps", "docker ls-run", "docker show", "docker images"}, 0},
		},
		Written: []Written{
			{"Explain what a CI/CD pipeline is and describe three benefits it brings.", 5},
			{"Compare containers with virtual machines in terms of isolation, size and start-up time.", 4},
		},
	},
}
