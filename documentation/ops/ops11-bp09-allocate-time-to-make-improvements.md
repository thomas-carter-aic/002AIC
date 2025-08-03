[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS11-BP09 Allocate time to make improvements

Dedicate time and resources within your processes to make continuous incremental improvements possible.

**Desired outcome:**

* You create temporary duplicates of environments, which lowers the risk, effort, and cost of experimentation and testing.

* These duplicated environments can be used to test the conclusions from your analysis, experiment, and develop and test planned improvements.

* You run gamedays, and you use Fault Injection Service (FIS) to provide the controls and guardrails that teams need to run experiments in a production-like environment.

**Common anti-patterns:**

* There is a known performance issue in your application server. It is added to the backlog behind every planned feature implementation. If the rate of planned features being added remains constant, the performance issue would never be addressed.

* To support continual improvement, you approve administrators and developers using all their extra time to select and implement improvements. No improvements are ever completed.

* Operational acceptance is complete, and you do not test operational practices again.

**Benefits of establishing this best practice:** By dedicating time and resources within your processes, you can make continuous, incremental improvements possible.

**Level of risk exposed if this best practice is not established:** Low

## Implementation guidance

* Allocate time to make improvements: Dedicate time and resources within your processes to make continuous, incremental improvements.

* Implement changes to improve and evaluate the results to determine success.

* If the results do not satisfy the goals and the improvement is still a priority, pursue alternative courses of action.

* Simulate production workloads through game days, and use learnings from these simulations to improve.

## Resources

**Related best practices:**

* [OPS05-BP08 Use multiple environments](https://docs.aws.amazon.com/wellarchitected/latest/operational-excellence-pillar/ops_dev_integ_multi_env.html)

**Related videos:**

* [AWS re:Invent 2023 - Improve application resilience with AWS Fault Injection Service](https://youtu.be/N0aZZVVZiUw?si=ivYa9ScBfHcj-IAq)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS11-BP08 Document and share lessons learned

Security

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>