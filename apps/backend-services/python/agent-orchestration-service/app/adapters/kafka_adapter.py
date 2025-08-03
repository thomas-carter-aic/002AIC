import asyncio
from aiokafka import AIOKafkaProducer

class KafkaAdapter:
    def __init__(self, bootstrap_servers):
        self.producer = AIOKafkaProducer(bootstrap_servers=bootstrap_servers)

    async def send_event(self, topic, event):
        await self.producer.start()
        try:
            await self.producer.send_and_wait(topic, event.encode())
        finally:
            await self.producer.stop()
