[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you evaluate your Serverless application’s health?

# **OPS 1:** How do you evaluate your Serverless application’s health?

Evaluating your metrics, distributed tracing, and logging gives you insight into business and operational [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload."), and helps you understand which services should be optimized to improve your customer’s experience.

## Resources

[Amazon CloudWatch Metrics and Dimensions](https://docs.aws.amazon.com/en_pv/AmazonCloudWatch/latest/monitoring/aws-services-cloudwatch-metrics.html?ref=wellarchitected)  
 [AWS Personal Health Dashboard](https://aws.amazon.com/premiumsupport/technology/personal-health-dashboard/?ref=wellarchitected)  
 [Amazon CloudWatch Automated Dashboard](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Automatic_Dashboards_Focus_Service.html?ref=wellarchitected)  
 [AWS Serverless Monitoring Partners](https://aws.amazon.com/lambda/partners/?ref=wellarchitected&partner-solutions-cards.sort-by=item.additionalFields.partnerName&partner-solutions-cards.sort-order=asc&awsf.partner-solutions-filter-partner-type=use-case%23monitoring&ref=wellarchitected)  
 [re:Invent 2019 - Production-grade full-stack apps with AWS Amplify](https://www.youtube.com/watch?v=DcrtvgaVdCU&ref=wellarchitected)

## Best Practices:

* **Understand, analyze, and alert on metrics provided out of the box**: Each managed service emits metrics out of the box. Establish key metrics for each managed service as the basis for comparison, and for identifying under and over performing [[components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.")](https://wa.aws.amazon.com/wat.concept.component.en.html?ref=wellarchitected-ws). Examples of key metrics include function errors, queue depth, failed state machine executions, and response times.

* **Use application, business, and operations metrics**: Identify key performance indicators (KPIs) based on desired business and customer outcomes. Evaluate KPIs to determine [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") success and operational health.

* **Use distributed tracing and code is instrumented with additional context**: Instrument your application code to emit information about its status, correlation identifiers, business outcomes, and information to determine transaction flows across your [[workload](serv.concept.workload.en.html "The set of components that together deliver business value.")](https://wa.aws.amazon.com/wat.concept.workload.en.html?ref=wellarchitected-ws).

* **Use structured and centralized logging**: Standardize your application logging to emit operational information about transactions, correlation identifiers, request identifiers across [[components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.")](https://wa.aws.amazon.com/wat.concept.component.en.html?ref=wellarchitected-ws), and business outcomes. Use this information to answer arbitrary questions about the state of your [[workload](serv.concept.workload.en.html "The set of components that together deliver business value.")](https://wa.aws.amazon.com/wat.concept.workload.en.html?ref=wellarchitected-ws).

## Improvement Plan

**Understand, analyze, and alert on metrics provided out of the box**  

* Understand what metrics and dimensions each managed service utilized provides out of the box
  * Use [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") per Service/Cross Service auto-generated dashboard to quickly visualize key metrics for each AWS service you use.  
[Amazon CloudWatch automated Dashboard](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Automatic_Dashboards_Focus_Service.html?ref=wellarchitected)  
[Amazon CloudWatch Cross Service Dashboard](https://docs.aws.amazon.com/en_pv/AmazonCloudWatch/latest/monitoring/CloudWatch_Automatic_Dashboards_Cross_Service.html?ref=wellarchitected)
  

* Configure alerts on relevant metrics to engage you when [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") are unhealthy.
  * <u>Examples</u>

    * For [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app."), alert on Duration, Errors, Throttling, and ConcurrentExecutions. For stream-based invocations, alert on IteratorAge. For Asynchronous invocations, alert on DeadLetterErrors.

    * For [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), IntegrationLatency, [Latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response."), 5XXError

    * For Application Load Balancer, HTTPCode\_ELB\_5XX\_Count, RejectedConnectionCount, HTTPCode\_Target\_5XX\_Count, UnHealthyHostCount, LambdaInternalError, LambdaUserError

    * For [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features."), 5XX and [Latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response.")

    * For [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers."), ApproximateAgeOfOldestMessage

    * For [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Streams, ReadProvisionedThroughputExceeded, WriteProvisionedThroughputExceeded, GetRecords.IteratorAgeMilliseconds, PutRecord.Success, PutRecords.Success (if using Kinesis Producer Library) and GetRecords.Success

    * For [Amazon SNS](serv.concept.sns.en.html "A web service that enables applications, end-users, and devices to instantly send and receive notifications from the cloud."), NumberOfNotificationsFailed, NumberOfNotificationsFilteredOut-InvalidAttributes

    * For [Amazon SES](serv.concept.ses.en.html "An easy-to-use, cost-effective email solution for applications."), Rejects, Bounces, Complaints, Rendering Failures

    * For [AWS Step Functions](serv.concept.awsstepfunctions.en.html "A web service that coordinates the components of distributed applications as a series of steps in a visual workflow."), ExecutionThrottled, ExecutionsFailed, ExecutionsTimedOut

    * For Amazon EventBridge, FailedInvocations, ThrottledRules

    * For [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web."), 5xxErrors, TotalRequestLatency

    * For [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability."), ReadThrottleEvents, WriteThrottleEvents, SystemErrors, ThrottledRequests, UserErrors
  
  * For metrics that have a discernible pattern, trend, has a minimal set of missing data points or that are not key to alert on one-time [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload."), alert based on [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") Anomaly expected values.  
[AWS Lambda CloudWatch Metrics](https://docs.aws.amazon.com/lambda/latest/dg/monitoring-functions-metrics.html?ref=wellarchitected)  
[Amazon API Gateway CloudWatch Metrics](https://docs.aws.amazon.com/apigateway/latest/developerguide/monitoring-cloudwatch.html?ref=wellarchitected)  
[AWS Application Load Balancer CloudWatch Metrics](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-cloudwatch-metrics.html?ref=wellarchitected)  
[AWS AppSync CloudWatch Metrics](https://docs.aws.amazon.com/appsync/latest/devguide/monitoring.html?ref=wellarchitected)  
[Amazon SQS CloudWatch Metrics](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-available-cloudwatch-metrics.html?ref=wellarchitected)  
[Amazon Kinesis Data Streams CloudWatch Metrics](https://docs.aws.amazon.com/streams/latest/dev/monitoring-with-cloudwatch.html?ref=wellarchitected)  
[Amazon SNS CloudWatch Metrics](https://docs.aws.amazon.com/sns/latest/dg/sns-monitoring-using-cloudwatch.html?ref=wellarchitected)  
[Amazon SES CloudWatch Metrics](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/event-publishing-retrieving-cloudwatch.html?ref=wellarchitected)  
[AWS Step Functions CloudWatch Metrics](https://docs.aws.amazon.com/step-functions/latest/dg/procedure-cw-metrics.html?ref=wellarchitected)  
[Amazon EventBridge CloudWatch Metrics](https://docs.aws.amazon.com/eventbridge/latest/userguide/eventbridge-monitoring-cloudwatch-metrics.html?ref=wellarchitected)  
[Amazon S3 CloudWatch Metrics](https://docs.aws.amazon.com/AmazonS3/latest/dev/cloudwatch-monitoring.html?ref=wellarchitected)  
[Amazon DynamoDB CloudWatch Metrics](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/metrics-dimensions.html?ref=wellarchitected)  
[Creating a CloudWatch Alarm based on a static threshold](https://docs.aws.amazon.com/en_pv/AmazonCloudWatch/latest/monitoring/ConsoleAlarms.html?ref=wellarchitected)  
[Creating a CloudWatch Alarm based on Metric Math expressions](https://docs.aws.amazon.com/en_pv/AmazonCloudWatch/latest/monitoring/Create-alarm-on-metric-math-expression.html?ref=wellarchitected)  
[Example alerts programmatically created via AWS CloudFormation](https://github.com/awslabs/realworld-serverless-application/blob/master/ops/sam/app/alarm.template.yaml?ref=wellarchitected)  
[Creating a CloudWatch Alarm based on Anomaly detection](https://docs.aws.amazon.com/en_pv/AmazonCloudWatch/latest/monitoring/Create_Anomaly_Detection_Alarm.html?ref=wellarchitected)
  

**Use application, business, and operations metrics**  

* Identify user journeys and metrics that can be derived from each customer transaction.
  * This exercise provides a mechanism to decide what metrics can be created programmatically and what to alert on. This result provides a more complete picture of your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.")’s health impact on business.
  

* Create custom metrics asynchronously as opposed to synchronously for improved performance, cost, and [reliability](serv.concept.c-reliability.en.html "A measure of your workload's ability to provide functionality when desired by the user.") outcomes.  
 [Creating Custom Metrics Asynchronously with Amazon CloudWatch](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation.html?ref=wellarchitected)

* Emit business metrics from within your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") to measure application performance against business goals.
  * <u>Examples</u>

    * `Number of orders created, payment operations, number of reservations,`etc.
  

* Create and analyze [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") metrics to measure interactions with upstream and downstream [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.").
  * <u>Examples</u>

    * `Message queue length, integration [latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response."), throttling, etc.`
  

* Create and analyze operational metrics to assess the health of your [continuous delivery](serv.concept.continuous_delivery.en.html "A software development practice in which code changes are automatically built, tested, and prepared for a release to production.") pipeline and operational processes.
  * <u>Examples</u>

    * `CI/CD feedback time, mean-time-between-failure, mean-time-between-recovery, number of on-call pages and time to resolution, etc.`
  

**Use distributed tracing and code is instrumented with additional context**  

* Identify common business contexts and system data that are commonly present across multiple transactions.
  * Use [AWS X-Ray](serv.concept.awsxray.en.html "A web service that collects data about requests that your application serves, and provides tools you can use to view, filter, and gain insights into that data to identify issues and opportunities for optimization.") annotations or trusted third-party tracing providers labels to easily group or filter traces, for example a customer ID, payment ID or state machine execution ID.  
[Adding annotations and metadata - AWS X-Ray Python SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-python-segment.html?ref=wellarchitected)  
[Adding annotations and metadata - AWS X-Ray Node.js SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-nodejs-segment.html?ref=wellarchitected)  
[Adding annotations and metadata - AWS X-Ray Java SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-java-segment.html?ref=wellarchitected)  
[Adding annotations and metadata - AWS X-Ray Go SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-go-segment.html?ref=wellarchitected)  
[Adding annotations and metadata - AWS X-Ray Ruby SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-ruby-segment.html?ref=wellarchitected)  
[Adding annotations and metadata - AWS X-Ray .NET SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-dotnet-segment.html?ref=wellarchitected)
  

* Instrument SDKs and requests to upstream/downstream services to understand the flow of a transaction across system.
  * This will help determine [latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response.") distribution, response times, number of retries, response codes, and exceptions.
  * Use labels/annotations to inject business context for requests made to system and third party to help filter and compare traces across [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.").
  * With [AWS X-Ray](serv.concept.awsxray.en.html "A web service that collects data about requests that your application serves, and provides tools you can use to view, filter, and gain insights into that data to identify issues and opportunities for optimization.") SDK or trusted third-party tracing provider, you can automatically instrument AWS SDKs including popular HTTP and database libraries.  
[Instrumenting downstream calls - AWS X-Ray Python SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-python-patching.html?ref=wellarchitected)  
[Instrumenting downstream calls - AWS X-Ray Node.js SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-nodejs-awssdkclients.html?ref=wellarchitected)  
[Instrumenting downstream calls - AWS X-Ray Java SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-java-awssdkclients.html?ref=wellarchitected)  
[Instrumenting downstream calls - AWS X-Ray Go SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-go-awssdkclients.html?ref=wellarchitected)  
[Instrumenting downstream calls - AWS X-Ray Ruby SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-ruby-awssdkclients.html?ref=wellarchitected)  
[Instrumenting downstream calls - AWS X-Ray .NET SDK](https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-dotnet-sdkclients.html?ref=wellarchitected)
  

**Use structured and centralized logging**  

* Log request identifiers from downstream services, [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") name, [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") runtime information, unique correlation identifiers and information that helps identify a business transaction.
  * <u>Examples</u>

    * Use `correlation_id, request_id, customer_id, service_name, timestamp, function_arn, function_memory` as [JSON](serv.concept.json.en.html "JavaScript Object Notation. A lightweight data interchange format. For information about JSON, see http://www.json.org/.") keys.
  

* Use [JSON](serv.concept.json.en.html "JavaScript Object Notation. A lightweight data interchange format. For information about JSON, see http://www.json.org/.") as your logging output. Prefer logging entire objects/dictionaries rather than many one line messages. Mask or remove sensitive data when logging.
  * <u>Examples</u>

    * `logging.info({“operation”: “cancel_booking”, “details”: result...})`
  

* Minimize logging debugging information as they can incur both costs and increase noise to signal ratio.
  * Sampling is an efficient mechanism you can use to set log level to debug for a percentage of requests in your logging framework while maintaining default log level to info.  
[Serverless Hero: Example of using structure logging with AWS Lambda](https://theburningmonk.com/2018/01/you-need-to-use-structured-logging-with-aws-lambda/?ref=wellarchitected)
  
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