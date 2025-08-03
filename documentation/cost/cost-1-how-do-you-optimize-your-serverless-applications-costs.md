[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you optimize your Serverless application’s costs?

# **COST 1:** How do you optimize your Serverless application’s costs?

Design, implement, and optimize your application to maximize value. Asynchronous design patterns and performance practices ensure efficient resource use and directly impact the value per business transaction.

## Resources

[AWS Lambda Best Practices](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html?ref=wellarchitected)  
 [re:Invent 2019 - Serverless architectural patterns and best practices](https://www.youtube.com/watch?v=9IYpGTS7Jy0&ref=wellarchitected)  
 [re:Invent 2019 - Optimizing your Serverless applications](https://www.youtube.com/watch?v=5rMiq-jw1Ig&ref=wellarchitected)

## Best Practices:

* **Minimize external calls and function code initialization**: Functions may call other managed services and third-party APIs to operate as intended. Functions may also use application dependencies that may not be suitable for ephemeral environments. Reviewing and optimizing both can directly impact on value provided per invocation.

* **Optimize logging output and its retention**: Review logging level, logging output, and log retention to ensure that they meet your operational needs. This helps prevent unnecessary logging and data retention while ensuring that you have the minimum levels necessary for [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") operation.

* **Use cost-aware usage patterns in code**: Reduce the time consumed by running functions by eliminating job-polling or task coordination.

* **Optimize function configuration to reduce cost**: Functions unit of scale is [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") where CPU, Network and I/O are proportionately allocated. Consider benchmarking and reviewing whether you are under/overutilising what your function is allocated to. Benchmark your Lambda function execution with various [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") settings as under some conditions the added [Memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.")/CPU may lower the duration and with this new combination reduce the cost of each invocation.

## Improvement Plan

**Minimize external calls and function code initialization**  

* Review code initialization.
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") reports the time it takes to initialize application code in [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Logs. Consider reviewing application code and its dependencies to improve overall execution to maximise on value.
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") reuses execution contexts, meaning that you can take advantage of making external calls to resources where their results might be used more than once over a period of time.
  * Use TTL mechanisms inside your function handler to ensure you can prevent additional external calls that incur additional execution time while preemptively fetching non-stale data.  
[AWS Lambda Execution Context](https://docs.aws.amazon.com/lambda/latest/dg/running-lambda-code.html?ref=wellarchitected)
  

* Review third-party application deployments and permissions.
  * When using [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") Layers or Serverless Applications provisioned by AWS Serverless Applications Repository, be sure to understand any associated charges that these may incur.
  * Ensure that your Lambda function only has access to what its application code needs, and regularly review if your function has a predicted usage pattern (i.e [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web."), [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability.") tables).
  

**Optimize logging output and its retention**  

* Emit and capture only what is necessary to understand and operate your [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") as intended.
  * With [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app."), any standard output statements are ingested into [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Logs.
  * Capture and emit business and operational [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") that are necessary to help you understand your function, its integration, and its interactions.
  * Use a logging framework and environment variables to dynamically set logging level, and when applicable, sample debugging logs for a percentage of invocations.
  * With [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), enable access logs, and selectively review its output format and request fields that might be necessary.
  * With [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features."), enable access logs for debugging purposes.  
[Setting up Access Logs in AWS AppSync](https://docs.aws.amazon.com/appsync/latest/devguide/monitoring.html?ref=wellarchitected)  
[Setting up Access Logs in Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-logging.html?ref=wellarchitected)
  

* Define and set a log retention strategy to meet your operational and business needs.
  * Set log expiration for each [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Log Group as they are kept indefinitely by default. For log archival, [CloudWatch Logs](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") can be exported to [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web.") and stored in [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web.") Glacier for more cost-effective retention. Alternatively, you can use [CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Log Subscriptions to filter, pre-process, or ship log entries to trusted third-party providers.
  * Archive and export [CloudWatch Logs](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") to [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web."), and store in [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web.") Glacier for more cost-effective retention where applicable.
  * Alternatively, you can use [CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Log Subscriptions to filter, pre-process, or ship log entries to trusted third-party providers.  
[Real-time Processing of Log Data with Subscriptions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Subscriptions.html?ref=wellarchitected)  
[Working with Log Groups and Log Streams](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Working-with-log-groups-and-streams.html?ref=wellarchitected)  
[Example: Automatically setting Amazon CloudWatch Logs retention](https://serverlessrepo.aws.amazon.com/applications/arn:aws:serverlessrepo:us-east-1:374852340823:applications~auto-set-log-group-retention?ref=wellarchitected)  
[Exporting Amazon CloudWatch Logs to Amazon S3](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/S3Export.html?ref=wellarchitected)
  

**Use cost-aware usage patterns in code**  

* Decide whether your application can fit an async pattern.
  * Refrain from introducing idleness where your [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions might be waiting for external activities to complete.
  * With [AWS Step Functions](serv.concept.awsstepfunctions.en.html "A web service that coordinates the components of distributed applications as a series of steps in a visual workflow."), you can poll for the status of tasks more efficiently. Long polling or waiting has the effect of increasing the costs of Lambda functions as they are waiting idle and at the same time reducing overall account concurrency, potentially impacting the ability of other functions to run.
  * With [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics."), create custom metrics asynchronously when possible by using [CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Embedded Metric Format.
  * Alternatively, use canonical log lines and a log subscription to convert canonical lines into metrics. Creating metrics synchronously within your [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function impact on overall execution time, and has a direct correlation with costs.
  

* Consider asynchronous invocations and review run away functions where applicable.
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions can be triggered based on [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") ingested into [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") queues, [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web.") buckets, [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") streams, where AWS manages the polling infrastructure on your behalf with no additional cost.
  * Use EventBridge to integrate with SaaS as opposed to polling for third-party software as a service (SaaS) providers.
  * Carefully consider and review recursion, and establish timeouts to prevent run away functions.  
[Ten Things Serverless Architects should know](https://aws.amazon.com/blogs/architecture/ten-things-serverless-architects-should-know/?ref=wellarchitected)
  

**Optimize function configuration to reduce cost**  

* Benchmark your function using different [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") sizes.
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") proportionally allocates CPU, Network, and I/O to the [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") allocated. Benchmark your [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions with differing amounts of [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") allocated, and review and compare [memory](serv.concept.memory.en.html "A component of a computer system that is designed for short-term, fast-access, data storage; often this is Random-Access Memory (RAM), but there are other forms as well.") settings that reduce your overall execution to ensure you have the most cost-effective invocation.
  
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