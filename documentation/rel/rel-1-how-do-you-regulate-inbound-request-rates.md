[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you regulate inbound request rates?

# **REL 1:** How do you regulate inbound request rates?

Defining, analyzing, and enforcing inbound request rates helps achieve better throughput. Regulation helps you adapt different scaling mechanisms based on customer demand.

## Resources

[Account Level Throttling](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html?ref=wellarchitected)  
 [Amazon API Gateway Limits](https://docs.aws.amazon.com/apigateway/latest/developerguide/limits.html?ref=wellarchitected)

## Best Practices:

* **Use throttling to control inbound request rates**: Use throttling limits to control inbound requests by setting steady-state and burst rates limits.

* **Use mechanisms to protect non-scalable resources**: Functions can scale faster than traditional resources, such as [relational databases](serv.concept.relational.en.html "A relational database is a collection of data items with pre-defined relationships between them.") and [cache](serv.concept.cache.en.html "A place that data is stored, temporarily, to increase performance by decreasing access time to frequently used data.") systems. Protect non-scalable resources by adapting fast scaling [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") to downstream systems throughput.

* **Use, analyze, and enforce API quotas**: API quotas limit the maximum number of requests that can be submitted within a specified time interval with a given API key.

## Improvement Plan

**Use throttling to control inbound request rates**  

* Identify steady-rate and burst rate requests that your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") can sustain at any point in time before performance degraded.
  * Perform load testing for a sustained period of time, gradually increasing traffic to determine your steady-state rate of requests.
  * Use a burst strategy/no ramp up to determine the burst rates that your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") can serve without errors or performance degradation.  
[AWS Marketplace: Gatling FrontLine Load Testing](https://aws.amazon.com/marketplace/pp/Gatling-Corp-Gatling-FrontLine-High-Scale-Load-Tes/B07DTWPZG8?ref=wellarchitected)  
[Amazon Partner: BlazeMeter Load Testing](https://aws.amazon.com/partners/find/partnerdetails/?n=BlazeMeter%2C%20a%20CA%20Technologies%20Company&id=001E000000Rp5PcIAJ&ref=wellarchitected)  
[Amazon Partner: Apica Load Testing](https://aws.amazon.com/partners/find/partnerdetails/?n=Apica&id=001E000000Rp56sIAB&ref=wellarchitected)
  

* Throttle inbound request rates using steady-rate and burst rate requests.
  * Enable throttling for individual [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") APIs, API stages, or per method to improve overall performance across all APIs in your account. This restricts the overall request submissions so that they don't go past the account-level throttling limits.
  * You can also throttle requests by introducing [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Stream or [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") as a buffering layer.
  * [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") can limit the number of requests at the shard level while [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") can limit at the consumer level.
  * For [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions, [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") can effectively control concurrency at the shard level, meaning that a single shard will have a single concurrent invocation per second.  
[Throttle API requests for better throughput](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html?ref=wellarchitected)  
[Throttle API requests for better throughput](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html?ref=wellarchitected)  
[Amazon Kinesis Data Stream as an event source for AWS Lambda](https://docs.aws.amazon.com/lambda/latest/dg/with-kinesis.html?ref=wellarchitected)  
[Amazon SQS as an event source for AWS Lambda](https://docs.aws.amazon.com/lambda/latest/dg/with-sqs.html?ref=wellarchitected)  
[Serverless Hero: Distributing and Throttling events with queues and message filtering](https://www.jeremydaly.com/how-to-use-sns-and-sqs-to-distribute-and-throttle-events/?ref=wellarchitected)
  

**Use mechanisms to protect non-scalable resources**  

* Limit [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") throughput by enforcing how many transactions it can accept directly or via buffer mechanisms, such as queues and streams.
  * For [relational databases](serv.concept.relational.en.html "A relational database is a collection of data items with pre-defined relationships between them.") such as [Amazon RDS](serv.concept.amazonrelationaldatabaseservice.en.html "A web service that makes it easier to set up, operate, and scale a relational database in the cloud. It provides cost-efficient, resizable capacity for an industry-standard relational database and manages common database administration tasks."), you can limit the number of connections per user in addition to the global maximum number of connections.
  * [Cache](serv.concept.cache.en.html "A place that data is stored, temporarily, to increase performance by decreasing access time to frequently used data.") results and only connect and fetch data from databases when needed.
  * Adjust the maximum number of connections for caching systems, including a caching expiration mechanism to prevent serving stale records.
  * [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Streams control concurrency at shard level, meaning that a single shard has a single concurrent invocation, thus reducing downstream calls to non-scalable resources such as a traditional database.
  * [Amazon Kinesis](serv.concept.amazonkinesis.en.html "A platform for streaming data on AWS. Kinesis offers services that simplify the loading and analysis of streaming data.") Data Streams also supports batch windows up to 5 minutes and batch record sizes, whichever comes first will control how frequent invocations can occur.
  * Use [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") reserved concurrency feature on your function to both reserve and limit the maximum concurrency it can achieve, if necessary.  
[Caching implementation patterns and considerations](https://aws.amazon.com/caching/implementation-considerations/?ref=wellarchitected)  
[Serverless Hero: Managing database connections with AWS Lambda](https://www.jeremydaly.com/manage-rds-connections-aws-lambda/?ref=wellarchitected)  
[Reserving AWS Lambda function concurrency](https://docs.aws.amazon.com/lambda/latest/dg/per-function-concurrency.html?ref=wellarchitected)
  

**Use, analyze, and enforce API quotas**  

* Define whether your API consumers are end users or machines.

* Segregate API consumers steady-rate requests and their quota into multiple buckets or tiers.
  * [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") Usage Plans allow your API consumer to access selected APIs at agreed-upon request rates and quotas that meet their business requirements and budget constraints.
  * Create and attach API keys to usage plans to control access to certain API stages.
  * Extract utilization data from usage plans to analyze API usage on a per-API key basis, generate billing documents and determine whether your customers need higher or lower limits.
  * Have a mechanism to allow customers to pre-emptively request higher limits, so they can be proactive when they anticipate increased use of your APIs.
  * [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") Lambda Authorizers can dynamically associate API keys to a given request. This is ideal for scenarios where you don’t control API consumers or want to associate API keys based on your own criteria.  
[Create and use Usage Plans with API keys](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-api-usage-plans.html?ref=wellarchitected)  
[Usage Plan API key output from Lambda Authorizers](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-lambda-authorizer-output.html?ref=wellarchitected)
  
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