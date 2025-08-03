[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you build [resiliency](serv.concept.resiliency.en.html "The ability for a system to recover from a failure induced by load, attacks, and failures.") into your Serverless application?

# **REL 2:** How do you build resiliency into your Serverless application?

Evaluate scaling mechanisms for Serverless and non-Serverless resources to meet customer demand, and build [resiliency](serv.concept.resiliency.en.html "The ability for a system to recover from a failure induced by load, attacks, and failures.") to withstand partial and intermittent failures across dependencies.

## Resources

[The Amazon Builder's Library](https://aws.amazon.com/builders-library/?ref=wellarchitected)  
 [Optimizing AWS SDK for AWS Lambda](https://aws.amazon.com/premiumsupport/knowledge-center/lambda-function-retry-timeout-sdk/?ref=wellarchitected)  
 [AWS Lambda error and retry behavior](https://docs.aws.amazon.com/lambda/latest/dg/retries-on-errors.html?ref=wellarchitected)  
 [Serverless Hero: Production tips for working with Amazon Kinesis Data Streams](https://theburningmonk.com/2017/04/aws-lambda-3-pro-tips-for-working-with-kinesis-streams/?ref=wellarchitected)

## Best Practices:

* **Manage transaction, partial, and intermittent failures**: Transaction failures might occur when [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") are under high load. Partial failures can occur during batch processing, while intermittent failures might occur due to network or other transient issues.

* **Manage duplicate and unwanted [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.")**: Duplicate [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") can occur when a request is retried, multiple consumers process the same message from a queue or stream, or when a request is sent twice at different time intervals with the same parameters. Design your applications to process multiple identical requests to have the same effect as making a single request. [Events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") not adhering to your schema should be discarded.

* **Consider scaling patterns at burst rates**: In addition to your baseline performance, consider evaluating how your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") handle initial burst rates that may be expected or unexpected peaks.

* **Orchestrate long-running transactions**: Long-running transactions can be processed by one or multiple [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement."). Favor state machines for long-running transaction instead of handling them within application code in a single [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") or multiple synchronous dependency call chains.

## Improvement Plan

**Manage transaction, partial, and intermittent failures**  

* Use exponential backoff with jitter.
  * When responding to callers in fail fast scenarios, and under performance degradation, inform the caller via headers or metadata when they can retry. Amazon SDKs implement this by default.
  * Implement similar logic in your dependency layer when calling your own dependent services.
  * For downstream calls, adjust AWS and third-party SDK retries, backoffs, TCP and HTTP timeouts with your [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") timeout to help you decide when to stop retrying.  
[Error Retries and Exponential Backoff in AWS](https://docs.aws.amazon.com/general/latest/gr/api-retries.html?ref=wellarchitected)
  

* Use a dead-letter queue mechanism to retain, investigate, and retry failed transactions
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") allows failed transactions to be sent to a dedicated [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") dead-letter queue on a per function basis.
  * [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Stream and [Amazon DynamoDB Streams](serv.concept.dynamodb-streams.en.html "An AWS service that captures a time-ordered sequence of item-level modifications in any Amazon DynamoDB table, and stores this information in a log for up to 24 hours. Applications can access this log and view the data items as they appeared before and after they were modified, in near real time.") retry the entire batch of items. Repeated errors block processing of the affected shard until the error is resolved or the items expire.
  * Within [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app."), you can configure Maximum Retry Attempts, Maximum Record Age and Destination on Failure to respectively control retry while processing data records, and effectively remove poison-pill messages from the batch by sending its metadata to an [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") dead-letter queue for further analysis.  
[When to use dead-letter queues](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html?ref=wellarchitected)  
[Example: Serverless Application Repository DLQ redriver](https://serverlessrepo.aws.amazon.com/applications/arn:aws:serverlessrepo:us-east-1:303769779339:applications~aws-sqs-dlq-redriver?ref=wellarchitected)
  

**Manage duplicate and unwanted events**  

* Generate unique attributes needed to manage duplicate [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") at the beginning of the transaction.
  * Depending on the final destination, duplicate [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") might write to the same record with the same content instead of generating a duplicate entry, and therefore may not require additional safeguards.
  

* Use an external system, such as a database, to store unique attributes of a transaction that can be verified for duplicates.
  * These unique attributes, also known as idempotency tokens, can be business-specific, such as transaction ID, payment ID, booking ID, opaque random alphanumeric string, unique correlation identifiers, or the hash of the content.
  * Use [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability.") as a control database to store transactions and idempotency tokens.
    * <u>Examples</u>

      * Use a conditional write to fail a refund operation if a payment reference has already been refunded, thus signaling to the application that it is a duplicate transaction.

      * The application can then catch this exception and return the same result to the customer as if the refund was processed successfully.
  

* Validate [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") using a pre-defined and agreed upon schema.
  * [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions can use one or more [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") sources to trigger invocation. If [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") can be issues by external sources, your customers or machine generated, use a schema to validate your [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") conforms with what you’re expecting to process within your application code or at the [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") source when applicable.  
[Community Hero: Your AWS Lambda function might execute twice](https://cloudonaut.io/your-lambda-function-might-execute-twice-deal-with-it/?ref=wellarchitected)  
[Stripe Idempotent tokens generated by consumers](https://stripe.com/docs/api/idempotent_requests?ref=wellarchitected)  
[Setting Time-to-live for Amazon DynamoDB records/idempotent tokens](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/TTL.html?ref=wellarchitected)  
[Request Validation in Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-method-request-validation.html?ref=wellarchitected)  
[Matching events patterns in EventBridge](https://docs.aws.amazon.com/eventbridge/latest/userguide/eventbridge-and-event-patterns.html?ref=wellarchitected)  
[Filtering messages with Amazon SNS](https://docs.aws.amazon.com/sns/latest/dg/sns-message-filtering.html?ref=wellarchitected)  
[JSON Schema Implementations](https://json-schema.org/implementations.html?ref=wellarchitected)
  

**Consider scaling patterns at burst rates**  

* Perform load test using burst strategy with random intervals of idleness.
  * Load test using burst of requests for a short period of time and introduce burst delays to allow your [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") to recover from unexpected load. This allows you to future proof your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") for key [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") that you may not be certain of how much variance in requests you may receive.  
[AWS Marketplace: Gatling FrontLine Load Testing](https://aws.amazon.com/marketplace/pp/Gatling-Corp-Gatling-FrontLine-High-Scale-Load-Tes/B07DTWPZG8?ref=wellarchitected)  
[Amazon Partner: BlazeMeter Load Testing](https://aws.amazon.com/partners/find/partnerdetails/?n=BlazeMeter%2C%20a%20CA%20Technologies%20Company&id=001E000000Rp5PcIAJ&ref=wellarchitected)  
[Amazon Partner: Apica Load Testing](https://aws.amazon.com/partners/find/partnerdetails/?n=Apica&id=001E000000Rp56sIAB&ref=wellarchitected)
  

* Review service account limits with combined utilization across resources.
  * Review [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") account level limits such as number of requests per second across all APIs.
  * Review [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function concurrency reservations and ensure that there is enough pool capacity to allow other functions to scale.
  * Review [Amazon CloudFront](serv.concept.amazoncf.en.html "An AWS content delivery service that helps you improve the performance, reliability, and availability of your websites and applications.") requests per second per distribution.
  * Review [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.")@Edge requests per second and concurrency limit.
  * Review [AWS IoT](serv.concept.awsiot.en.html "A managed cloud platform that lets connected devices easily and securely interact with cloud applications and other devices.") Message Broker concurrent requests per second.
  * Review EventBridge API requests and target invocations limit.
  * Review [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices.") API limits.
  * Review [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability.") throughput, [indexes](serv.concept.indexes.en.html "A technology that is designed to make looking up information more efficient."), and request rates limits.  
[AWS AppSync throttle rate limits](https://docs.aws.amazon.com/general/latest/gr/aws_service_limits.html?ref=wellarchitected)  
[Amazon CloudFront requests rates limits](https://docs.aws.amazon.com/general/latest/gr/aws_service_limits.html?ref=wellarchitected)  
[AWS Lambda@Edge request rates limits](https://docs.aws.amazon.com/general/latest/gr/aws_service_limits.html?ref=wellarchitected)  
[AWS IoT Message Broker connections and requests limit](https://docs.aws.amazon.com/general/latest/gr/aws_service_limits.html?ref=wellarchitected)  
[EventBridge request rates limits](https://docs.aws.amazon.com/eventbridge/latest/userguide/cloudwatch-limits-eventbridge.html?ref=wellarchitected)  
[Amazon Cognito request rates limits](https://docs.aws.amazon.com/cognito/latest/developerguide/limits.html?ref=wellarchitected)  
[Amazon DynamoDB request rates and resources limit](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Limits.html?ref=wellarchitected)  
[AWS Services General Limits](https://docs.aws.amazon.com/general/latest/gr/aws_service_limits.html?ref=wellarchitected)
  

* Evaluate key metrics to understand how your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") recovers from bursts.
  * For [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app."), review Duration, Errors, Throttling, and ConcurrentExecutions and UnreservedConcurrentExecutions
  * For [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), review [Latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response."), IntegrationLatency 5xxError, 4xxError
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
  

**Orchestrate long-running transactions**  

* Use a state machine to provide a visual representation of distributed transactions, and to separate business logic from orchestration logic.
  * [AWS Step Functions](serv.concept.awsstepfunctions.en.html "A web service that coordinates the components of distributed applications as a series of steps in a visual workflow.") lets you coordinate multiple AWS services into Serverless workflows via state machines.
  * Within Step Functions, you can set separate retries, backoff rates, max attempts, intervals, and timeouts for every step of your state machine using a declarative language.  
[State Machine Error handling example](https://docs.aws.amazon.com/step-functions/latest/dg/concepts-error-handling.html?ref=wellarchitected)  
[Example AWS Step Functions State Machine via AWS SAM](https://github.com/aws-samples/aws-serverless-airline-booking/blob/develop/src/backend/booking/template.yaml?ref=wellarchitected)

    * <u>Examples</u>

      * The Refund-Flight function will be invoked only if the Allocate-Seat function fails and after three retried attempts with 1-second interval.
  

* Use dead-letter queues in response to failed state machine executions.
  * For high [durability](serv.concept.durability.en.html "The ability of a system to remain functional when faced with the challenges of normal operation over its lifetime.") within your state machines, use [AWS Step Functions](serv.concept.awsstepfunctions.en.html "A web service that coordinates the components of distributed applications as a series of steps in a visual workflow.") service integrations to send failed transactions to a dead letter queue of your choice as the final step.
  * For low [latency](serv.concept.latency.en.html "A measurement of the amount of time between an action and the result, often between a request and a response.") and no strict success rate requirements, you can use function composition with [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions calling other functions asynchronously.
  * Transactions that may fail will be retried at least twice depending on the [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") source and sent to each function’s dead-letter queue (for example, [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers."), [Amazon SNS](serv.concept.sns.en.html "A web service that enables applications, end-users, and devices to instantly send and receive notifications from the cloud.")).
  * Set alerts on the number of messages in the dead-letter queue, and either re-drive messages back to the workflow or disable parts of the workflow temporarily.  
[Sending failed transactions to Amazon SQS within Step Functions State Machine](https://docs.aws.amazon.com/step-functions/latest/dg/connect-sqs.html?ref=wellarchitected)  
[Serverless Hero: Function composition using asynchronous invocations](https://www.jeremydaly.com/the-dynamic-composer-an-aws-serverless-pattern/?ref=wellarchitected)
  
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