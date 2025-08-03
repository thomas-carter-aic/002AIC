[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you implement application Security in your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.")?

# **SEC 3:** How do you implement application Security in your workload?

Review and automate security practices at the application code level, and enforce security code review as part of development workflow. These [best practices](serv.concept.best-practice.en.html "Proven ways of achieving successful outcomes.") protect against emerging security threats and reduce the attack surface from malicious code (including third-party dependencies).

## Resources

[AWS Security Blog](https://aws.amazon.com/blogs/security/?ref=wellarchitected)  
 [OWASP Top 10 Serverless Interpretation](https://www.owasp.org/images/5/5c/OWASP-Top-10-Serverless-Interpretation-en.pdf?ref=wellarchitected)

## Best Practices:

* **Review security awareness documents frequently**: Stay up to date with both AWS and industry security [best practices](serv.concept.best-practice.en.html "Proven ways of achieving successful outcomes.") to understand and evolve protection of the [workload](serv.concept.workload.en.html "The set of components that together deliver business value.").

* **Store secrets that are used in your code securely**: Store your secrets such as database passwords or API keys in a Secrets Manager that allows for rotation, secure and audited access.

* **Automatically review [workload](serv.concept.workload.en.html "The set of components that together deliver business value.")’s code dependencies/libraries**: Regularly review of application and code dependencies is good industry security practice and helps you detect and prevent non-certified application code.

* **Validate inbound [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.")**: Sanitize inbound [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") and validate them against a predefined schema. Test your inputs by using fuzzing techniques.

* **Implement runtime protection to help prevent against malicious code execution**: Runtime protection enables you to disable features like spawning child processes, network access or local filesystem access in your Lambda functions.

## Improvement Plan

**Review security awareness documents frequently**  

* Review and subscribe to CVE and security bulletins.
  * Regularly review news feeds from multiple sources that are relevant to the technologies used in your [workload](serv.concept.workload.en.html "The set of components that together deliver business value."). Subscribe to notification services to be informed of critical threats in near-real time.  
[Common Vulnerabilities and Exposures (CVE)](https://cve.mitre.org/?ref=wellarchitected)  
[Amazon Partner: Serverless Application Security Top 10](https://github.com/puresec/sas-top-10?ref=wellarchitected)  
[AWS Security Bulletins](https://aws.amazon.com/security/security-bulletins/?ref=wellarchitected)  
[CISA Security Alerts](https://www.us-cert.gov/ncas/alerts?ref=wellarchitected)  
[NIST Security Alerts](https://nvd.nist.gov/?ref=wellarchitected)
  

**Store secrets that are used in your code securely**  

* Audit secrets access through a Secrets Manager.
  * Using secrets managers to manage application secrets such as database passwords or API keys allows you to update your secrets independently of code.  
[Using AWS Secrets Manager with Lambda](https://aws.amazon.com/blogs/security/how-to-securely-provide-database-credentials-to-lambda-functions-by-using-aws-secrets-manager/?ref=wellarchitected)  
[Auditing Secrets with AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/latest/userguide/monitoring.html?ref=wellarchitected)  
[Using Hashicorp Vault with AWS Lambda & Amazon API Gateway](https://learn.hashicorp.com/terraform/aws/lambda-api-gateway?ref=wellarchitected)  
[Auditing with Hashicorp Vault](https://www.vaultproject.io/docs/audit/?ref=wellarchitected)
  

* Enforce least privilege access to secrets.
  * By creating policies that enable minimal access to secrets, you prevent credentials from being accidentally used or being compromised.
  * Secrets that have policies that are too permissive could be misused by other environments or developers leading to accidental data loss or worse.  
[Authentication and Access Controls for AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/latest/userguide/auth-and-access.html?ref=wellarchitected)
  

* Rotate secrets frequently.
  * Rotating your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.")'s secrets frequently is important, it prevents your secrets from being misused since they will become invalid after each rotation.  
[Rotating your AWS Secrets Manager secrets](https://docs.aws.amazon.com/secretsmanager/latest/userguide/rotating-secrets.html?ref=wellarchitected)
  

**Automatically review workload’s code dependencies/libraries**  

* Implement security mechanisms to verify application code and its dependencies.
  * Combine automated with manual security code review process to examine application code and its dependencies to ensure they operate as intended.
  * Automated tools help identify overly complex application code, and common security vulnerability exposures already cataloged.
  * Manual security code reviews in addition to automated tools help ensure that application code works as intended end-to-end, including contextual business information and integrations that might not captured in automated tools.
  * Before adding any code dependencies to your [workload](serv.concept.workload.en.html "The set of components that together deliver business value."), you should take time to review, and certify each dependency to ensure that the code you’re adding to your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") is secure.
  * Use third-party services to automatically review your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.")’s dependencies on every commit, and periodically to ensure that your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") contains no known security vulnerabilities.  
[Snyk enables you to have automated security monitoring for dependencies.](https://snyk.io/?ref=wellarchitected)  
[OWASP Dependency Check](https://www.owasp.org/index.php/OWASP_Dependency_Check?ref=wellarchitected)  
[OWASP Security Code Review Guide 2.0](https://www.owasp.org/images/5/53/OWASP_Code_Review_Guide_v2.pdf?ref=wellarchitected)
  

**Validate inbound events**  

* Validate incoming HTTP requests against a schema
  * Enable [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") request validation to add another layer of security by ensuring the HTTP request matches the desired format and is rejected if it doesn’t match. Any HTTP request that doesn’t pass validation is rejected.
  * Implicitly trusting data from clients could lead to malformed data being processed.
  * Use data type validators or web application frameworks to ensure both semantic and data correctness, including but not limited to regular expressions, value range, data structure, data normalization, etc.  
[Input Validation for Serverless](https://www.secjuice.com/input-validation-serverless-security/?ref=wellarchitected)  
[Enable Request Validation in Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-method-request-validation.html?ref=wellarchitected)  
[Creating Models and Mapping Templates in Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/models-mappings.html?ref=wellarchitected)
  

**Implement runtime protection to help prevent against malicious code execution**  

* Lock out [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") Runtime functions when required.
  * When required, lock out features of the function runtime to prevent unauthorized actions that could arise from unverified code dependencies.
  * When applicable, [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") Custom Runtimes allows you to bring your own security tools that can intercept before and after each [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function runs. This enables you to centralize security mechanisms that are not required to be deployed within application code at the expense of managing and maintaining a function runtime.  
[Amazon Partner: PureSec’s FunctionShield](https://www.puresec.io/function-shield?ref=wellarchitected)
  
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