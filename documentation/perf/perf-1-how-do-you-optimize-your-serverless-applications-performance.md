[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you optimize your Serverless application’s performance?

# **PERF 1:** How do you optimize your Serverless application’s performance?

Evaluating and optimizing your Serverless application’s performance based on access patterns, scaling mechanisms, and native integrations allows you to continuously gain more value per transaction.

## Resources

[Serverless Hero: Reusing Database Connections in AWS Lambda](https://www.jeremydaly.com/reuse-database-connections-aws-lambda/?ref=wellarchitected)  
 [re:Invent 2019 - Best practices for AWS Lambda and Java](https://www.youtube.com/watch?v=ddg1u5HLwg8&ref=wellarchitected)  
 [re:Invent 2019 - I didn’t know Amazon API Gateway did that](https://www.youtube.com/watch?v=yfJZc3sJZ8E&ref=wellarchitected)  
 [re:Invent 2019 - Serverless at scale: Design patterns and optimizations](https://www.youtube.com/watch?v=dzU_WjobaRA&ref=wellarchitected)

## Best Practices:

* **Measure, evaluate, and select optimum capacity units**: Capacity units can be function [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") size, stream shards, database reads/writes request units, API endpoints, etc.

* **Integrate with managed services directly over functions when possible**: Consider using native integration between managed services as opposed to functions when no custom logic or data transformation is required.

* **Measure and optimize function startup time**: Evaluate function startup time for both performance and cost.

* **Take advantage of concurrency via async and stream-based function invocations**: Functions can be executed synchronously and asynchronously. A functions’ concurrency model can be better used via asynchronous and stream-based invocations. Queues, streams, or [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") can be aggregated resulting in more efficient processing time per invocation rather than invocation per request-response approach.

* **Optimize access patterns and apply caching where applicable**: Consider caching data that may not need to be frequently up-to-date, and optimize access patterns to only fetch data that is necessary to end users.

## Improvement Plan

**Measure, evaluate, and select optimum capacity units**  

* Identify and implement optimum capacity units.
  * For [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions, the Lambda Power Tuning application can help you systematically test different [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") size configurations and depending on your performance strategy - cost, performance, balanced - it identifies what is the most optimum [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") size to use.
  * For [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability."), On-Demand Instance can support up to 40,000 read/write request units per second. It is recommended for unpredictable application traffic and new tables with unknown [workloads](serv.concept.workload.en.html "The set of components that together deliver business value."). For higher and predictable throughputs, provisioned capacity along with DynamoDB automatic scaling is recommended over On-Demand Instance.
  * For high throughput [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Streams with multiple consumers, consider using Enhanced Fan-Out for dedicated 2-MiB throughput per consumer. When possible, use Kinesis Producer Library and Kinesis Consumer Library for effective record aggregation and de-aggregation.
  * For [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), you can use Edge endpoint for geographically distributed clients, Regional endpoints when clients are in the same Region, and Private endpoints for when API consumers should access your API via your [Amazon Virtual Private Cloud](serv.concept.amazonvirtualprivatecloud.en.html "A web service for provisioning a logically isolated section of the AWS cloud where you can launch AWS resources in a virtual network that you define. You control your virtual networking environment, including selection of your own IP address range, creation of subnets, and configuration of route tables and network gateways.") (VPC).
  * Performance load testing is recommended at both sustained and burst rates.
  * Use [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Service Dashboards to analyze key performance metrics including load testing results to evaluate the effect of tuning capacity units.  
[Understanding when to use Amazon DynamoDB On-Demand Instance and provisioned capacity.](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.ReadWriteCapacityMode.html?ref=wellarchitected)  
[Amazon Kinesis Data Streams Enhanced Fan-Out](https://docs.aws.amazon.com/streams/latest/dev/introduction-to-enhanced-consumers.html?ref=wellarchitected)  
[Choose an Amazon API Gateway Endpoint type](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-api-endpoint-types.html?ref=wellarchitected)  
[AWS X-Ray](https://docs.aws.amazon.com/xray/latest/devguide/aws-xray.html?ref=wellarchitected)  
[Analyzing Log Data with Amazon CloudWatch Logs Insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/AnalyzingLogData.html?ref=wellarchitected)
  

**Integrate with managed services directly over functions when possible**  

* Utilize native cloud service integrations.
  * When using [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") APIs, you can use AWS integration type to natively connect with other AWS services. In this integration type, API Gateway uses Apache Velocity Templates (VTL) and HTTPS to directly integrate with other AWS services and timeouts and errors are expected to be managed by the API consumer.
  * When using [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features."), you can use both Apache Velocity Templates (VTL), direct integration with [Amazon Aurora](serv.concept.aurora.en.html "A fully managed MySQL-compatible relational database engine that combines the speed and availability of commercial databases with the simplicity and cost-effectiveness of open source databases."), OpenSearch and any publicly available HTTP endpoint. [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") can also utilize multiple integrations and maximize throughput at data field level.
    * <u>Examples</u>

      * Full-text searches on field `orderDescription` are executed against [Amazon OpenSearch](serv.concept.amazonopensearchservice.en.html "An AWS managed service for deploying, operating, and scaling OpenSearch, an open-source search and analytics engine, in the AWS Cloud. Amazon OpenSearch Service also offers security options, high availability, data durability, and direct access to the OpenSearch APIs.") while remaining data is fetched from [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability.").
  * For State Machines managed by [AWS Step Functions](serv.concept.awsstepfunctions.en.html "A web service that coordinates the components of distributed applications as a series of steps in a visual workflow."), you can use Service Integrations feature to fetch and put data into [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability."), run an AWS Batch job, publish messages to [Amazon SNS](serv.concept.sns.en.html "A web service that enables applications, end-users, and devices to instantly send and receive notifications from the cloud.") topics, send messages to [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") queues, etc.
  * For [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.")\-driven use cases, EventBridge can connect to various AWS services natively, and act as an [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") bus across multiple AWS accounts to ease integration.  
[Amazon API Gateway Apache Velocity Template Reference](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-mapping-template-reference.html?ref=wellarchitected)  
[Integrating multiple with data sources with AWS AppSync](https://docs.aws.amazon.com/appsync/latest/devguide/tutorials.html?ref=wellarchitected)  
[Integrating with AWS Services via Step Functions Service Integrations](https://docs.aws.amazon.com/step-functions/latest/dg/concepts-service-integrations.html?ref=wellarchitected)  
[EventBridge and supported targets](https://docs.aws.amazon.com/eventbridge/latest/userguide/what-is-amazon-eventbridge.html?ref=wellarchitected)
  

**Measure and optimize function startup time**  

* Analyze and improve startup time.
  * Use [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function code initialization time reported in [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Logs (Init duration) or [AWS X-Ray](serv.concept.awsxray.en.html "A web service that collects data about requests that your application serves, and provides tools you can use to view, filter, and gain insights into that data to identify issues and opportunities for optimization.") to measure startup time that can be improved.
    * <u>Examples</u>

      * For a Python function, use `PYTHONPROFILEIMPORTTIME=1` environment variable to profile and understand what packages impact startup time
  * Prefer simpler frameworks that load quickly on execution context startup.  
[Serverless Hero: Lambda API framework](https://www.jeremydaly.com/projects/lambda-api/?ref=wellarchitected)  
[MiddyJS framework](https://middy.js.org/?ref=wellarchitected)  
[Python Chalice framework](https://github.com/aws/chalice/?ref=wellarchitected)  
[Performance improvement configuration - AWS Java SDK](https://docs.aws.amazon.com/sdk-for-java/v2/developer-guide/client-configuration-starttime.html?ref=wellarchitected)

    * <u>Examples</u>

      * Prefer simpler Java dependency injection frameworks like Dagger or Juice, over more complex like Spring.

      * Favor lightweight web frameworks optimized for [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") like MiddyJS, Lambda API JS, Python Chalice over Node.js Express, Python Django, or Flask.
  

* Take advantage of execution context reuse to improve the performance of your function.
  * Initialize SDK clients and database connections outside of the function handler and [cache](serv.concept.cache.en.html "A place that data is stored, temporarily, to increase performance by decreasing access time to frequently used data.") static assets locally in the /tmp directory. Subsequent invocations processed by the same instance of your [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function can reuse these resources.  
[Understanding AWS Lambda Execution Context](https://docs.aws.amazon.com/lambda/latest/dg/running-lambda-code.html?ref=wellarchitected)  
[Serverless Hero: Enable HTTP Keep Alive - AWS Node.js SDK](https://theburningmonk.com/2019/02/lambda-optimization-tip-enable-http-keep-alive/?ref=wellarchitected)
  

* Minimize your deployment package size to only its runtime necessities.
  * Only bring dependencies that are necessary to your application, and when available use code bundling to reduce file system lookup calls impact including its deployment package size.  
[Serverless Hero: Optimizing AWS Node.js SDK imports](https://theburningmonk.com/2019/03/just-how-expensive-is-the-full-aws-sdk/?ref=wellarchitected)
  

**Take advantage of concurrency via async and stream-based function invocations**  

* Favor asynchronous over synchronous request-response processing.
  * Asynchronous [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function invocations are sent to a queue, and an external process separate from the function manages polling and retries including exponential backoff out of the box.
  * Asynchronous invocations support dead-letter queues that can be configured at a per function level - Dead-letter queues may be an [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") queue or an [Amazon SNS](serv.concept.sns.en.html "A web service that enables applications, end-users, and devices to instantly send and receive notifications from the cloud.") topic.
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") service sends the async [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") to a dead-letter queue if it’s unable to receive a successful response from Lambda in up to three attempts. For invocations that may not succeed due to throttling (HTTP 429) or system errors (HTTP 500-series), Lambda service retries invoking the function up to 6 hours.  
[Understanding asynchronous invocation model for AWS Lambda](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async.html?ref=wellarchitected)
  

* Tune batch size, batch window and compress payloads for high throughput.
  * You can configure a batch window to buffer streaming records for up to 5 minutes, or you can set a limit of how many maximum records Lambda can process by setting a batch size - Your Lambda function will be invoked as to whichever comes first.
  * For high volume throughput, you can increase [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Streams shards resulting in increased concurrency at the expense of ordering (per shard). Additionally, Kinesis Enhanced Fan-Out can maximise throughput by dedicating a 2 MiB input/output channel per second per consumer instead of 2 MiB per shard.
  * For high volume and single consumer, you can use [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") as an [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") source for your Lambda function and process up to 1000 batch of records per second.
  * When possible, producers can compress records at the expense of additional CPU cycles for decompressing in your Lambda function code.  
[Using Amazon SQS queues and AWS Lambda for high throughput](https://docs.aws.amazon.com/lambda/latest/dg/with-sqs.html?ref=wellarchitected)  
[Understanding stream-based invocations with Amazon Kinesis and AWS Lambda](https://docs.aws.amazon.com/lambda/latest/dg/with-kinesis.html?ref=wellarchitected)  
[Increasing stream processing performance with Enhanced Fan-Out and AWS Lambda](https://aws.amazon.com/blogs/compute/increasing-real-time-stream-processing-performance-with-amazon-kinesis-data-streams-enhanced-fan-out-and-aws-lambda/?ref=wellarchitected)
  

**Optimize access patterns and apply caching where applicable**  

* Implement caching for suitable access patterns.
  * For REST APIs, you can use [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") Caching to reduce the number of calls made to your endpoint and also improve the [latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response.") of requests to your API.
  * For geographically distributed clients, [Amazon CloudFront](serv.concept.amazoncf.en.html "An AWS content delivery service that helps you improve the performance, reliability, and availability of your websites and applications.") or your third-party trusted CDN can [cache](serv.concept.cache.en.html "A place that data is stored, temporarily, to increase performance by decreasing access time to frequently used data.") results at the edge and further reducing network round-trip [latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response.").
  * For [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability."), you can enable caching with DynamoDB Accelerator (DAX) for use cases that may not require strongly consistent reads and are ready-intensive.
  * For GraphQL APIs, you can use [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") Server-side Caching at the API level. For queries with common arguments or a restrict set of arguments, you can also enable caching at the resolver level to improve overall responsiveness.
  * For general caching purposes, [Amazon ElastiCache](serv.concept.elasticache.en.html "A web service that simplifies deploying, operating, and scaling an in-memory cache in the cloud. The service improves the performance of web applications by providing information retrieval from fast, managed, in-memory caches, instead of relying entirely on slower disk-based databases.") supports a variety of caching patterns through [in-memory](serv.concept.in-memory.en.html "The state of being stored in volatile system RAM rather than on stable storage, such as flash or disk.") key-value stores like Redis and Memcached engines.
  * Define what is safe to be cached, TTL and set an eviction policy that fits your baseline performance, and access patterns to ensure that you do not serve stale record or [cache](serv.concept.cache.en.html "A place that data is stored, temporarily, to increase performance by decreasing access time to frequently used data.") data that should have a strongly consistent read.  
[Enabling Amazon API Gateway Caching](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-caching.html?ref=wellarchitected)  
[Use cases for Amazon DynamoDB Accelerator](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DAX.html?ref=wellarchitected)  
[Amazon ElastiCache caching and time-to-live strategies](https://docs.aws.amazon.com/AmazonElastiCache/latest/mem-ug/Strategies.html?ref=wellarchitected)  
[Serverless Hero: Caching Serverless Applications](https://theburningmonk.com/2019/10/all-you-need-to-know-about-caching-for-serverless-applications/?ref=wellarchitected)
  

* Reduce overfetching and underfetching
  * Utilize [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability.") queries over scan and utilize both Global Secondary Index (GSI) as well as composite sort keys to help you query hierarchical relationships in your data.
  * Consider [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") and GraphQL for interactive web applications, mobile, real-time, or use cases where data drives the User Interface. It can provide you data fetching flexibilities where your client can query only for the data it needs, in the format that it needs it in, however be mindful of too many nested queries where a response may take a couple of seconds possibly resulting in timeouts. Additionally, GraphQL helps you adapt access patterns as your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") evolves, thus making it more flexible to use purpose-built databases at any point in time.
  

* Compress payload and data storage
  * If your content supports deflate, gzip or identity content encoding, enable payload compression in [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.").
  * [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Firehose supports compressing streaming data using gzip, snappy, or zip. [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Firehose also support converting your streaming data from [JSON](serv.concept.json.en.html "JavaScript Object Notation. A lightweight data interchange format. For information about JSON, see http://www.json.org/.") to Apache Parquet or ORC. This can help improve performance and reduce data storage costs.  
[Best Practices when using Amazon Athena with AWS Glue](https://docs.aws.amazon.com/athena/latest/ug/glue-best-practices.html?ref=wellarchitected)  
[Enabling payload compression in Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-enable-compression.html?ref=wellarchitected)
  
[Sign In to the Console](https://console.aws.amazon.com/console/home?nc1=f_ct&src=footer-signin-mobile)

### Learn About AWS

* [What Is AWS?](https://aws.amazon.com/what-is-aws/?nc1=f_cc)
* [What Is Cloud Computing?](https://aws.amazon.com/what-is-cloud-computing/?nc1=f_cc)
* [What Is DevOps?](https://aws.amazon.com/devops/what-is-devops/?nc1=f_cc)
* [What Is a Container?](https://aws.amazon.com/containers/?nc1=f_cc)
* [What Is a Data Lake?](https://aws.amazon.com/big-data/datalakes-and-analytics/what-is-a-data-lake/?nc1=f_cc)
* [AWS Cloud Security](https://aws.amazon.com/security/?nc1=f_cc)
* [What's New](https://aws.amazon.com/new/?nc1=f_cc)
* [Blogs](https://aws.amazon.com/blogs/?nc1=f_cc)
* [Press Releases](https://press.aboutamazon.com/press-releases/aws "Press Releases")

### Resources for AWS

* [Getting Started](https://aws.amazon.com/getting-started/?nc1=f_cc)
* [Training and Certification](https://aws.amazon.com/training/?nc1=f_cc)
* [AWS Solutions Portfolio](https://aws.amazon.com/solutions/?nc1=f_cc)
* [Architecture Center](https://aws.amazon.com/architecture/?nc1=f_cc)
* [Product and Technical FAQs](https://aws.amazon.com/faqs/?nc1=f_dr)
* [Analyst Reports](https://aws.amazon.com/resources/analyst-reports/?nc1=f_cc)
* [AWS Partner Network](https://aws.amazon.com/partners/?nc1=f_dr)

### Developers on AWS

* [Developer Center](https://aws.amazon.com/developer/?nc1=f_dr)
* [SDKs& Tools](https://aws.amazon.com/developer/tools/?nc1=f_dr)
* [.NET on AWS](https://aws.amazon.com/developer/language/net/?nc1=f_dr)
* [Python on AWS](https://aws.amazon.com/developer/language/python/?nc1=f_dr)
* [Java on AWS](https://aws.amazon.com/developer/language/java/?nc1=f_dr)
* [PHP on AWS](https://aws.amazon.com/developer/language/php/?nc1=f_cc)
* [Javascript on AWS](https://aws.amazon.com/developer/language/javascript/?nc1=f_dr)

### Help

* [Contact Us](https://aws.amazon.com/contact-us/?nc1=f_m)
* [AWS Careers](https://aws.amazon.com/careers/?nc1=f_hi)
* [File a Support Ticket](https://console.aws.amazon.com/support/home/?nc1=f_dr)
* [Knowledge Center](https://aws.amazon.com/premiumsupport/knowledge-center/?nc1=f_dr)
* [AWS Support Overview](https://aws.amazon.com/premiumsupport/?nc1=f_dr)
* [Legal](https://aws.amazon.com/legal/?nc1=f_cc)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc1=f_ct&src=default)

Amazon is an Equal Opportunity Employer: *Minority / Women / Disability / Veteran / Gender Identity / Sexual Orientation / Age.*

* Language
* [English](https://wa.aws.amazon.com/index.en.html)
* [日本語](https://wa.aws.amazon.com/index.ja.html)
* [한국어](https://wa.aws.amazon.com/index.ko.html)
* [Français](https://wa.aws.amazon.com/index.fr.html)
* [Português](https://wa.aws.amazon.com/index.pt_BR.html)
* [Deutsch](https://wa.aws.amazon.com/index.de.html)
* [Español](https://wa.aws.amazon.com/index.es.html)
* [Italiano](https://wa.aws.amazon.com/index.it.html)
* [中文 (繁體)](https://wa.aws.amazon.com/index.zh_TW.html)
* [中文 (简体)](https://wa.aws.amazon.com/index.zh_CN.html)

* [Privacy](https://aws.amazon.com/privacy/?nc1=f_pr)
* |
* [Site Terms](https://aws.amazon.com/terms/?nc1=f_pr)
* |
* &copy; 2023, Amazon Web Services, Inc. or its affiliates. All rights reserved.