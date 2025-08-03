from langchain.agents import initialize_agent, load_tools
from langchain.llms import OpenAI
from crewai import Crew, Agent, Task
import asyncio

async def execute_workflow(request):
    llm = OpenAI(temperature=0)

    # Define agents/tasks (CrewAI)
    researcher = Agent(role='Researcher', goal='Find insights', backstory='Expert researcher', llm=llm)
    summarizer = Agent(role='Summarizer', goal='Summarize insights', backstory='Skilled summarizer', llm=llm)

    research_task = Task(description='Research the topic thoroughly', agent=researcher)
    summary_task = Task(description='Summarize the research findings', agent=summarizer)

    crew = Crew(agents=[researcher, summarizer], tasks=[research_task, summary_task])

    result = crew.kickoff()

    return {"summary": result}
