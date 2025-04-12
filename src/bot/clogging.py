import logging
from concurrent_log_handler import ConcurrentRotatingFileHandler
import threading
import queue

# Initialize a queue for logging
log_queue = queue.Queue()
_logging_setup_done = False  # Flag to ensure logging is setup only once

# Custom logging handler that writes messages from the queue to the file
class QueueHandler(logging.Handler):
    def __init__(self, log_queue):
        super().__init__()
        self.log_queue = log_queue

    def emit(self, record):
        self.log_queue.put(record)

# Custom logging listener that processes messages from the queue
class QueueListener(threading.Thread):
    def __init__(self, log_queue, handler):
        super().__init__()
        self.log_queue = log_queue
        self.handler = handler
        self.daemon = True
        self.start()

    def run(self):
        while True:
            record = self.log_queue.get()
            if record is None:
                break
            if record.msg == "Main script Loaded. Logging started...":
                self.handler.close()
                with open(self.handler.baseFilename, 'w', encoding='utf-8') as f:
                    f.write("")  # Truncate the file
                self.handler.stream = open(self.handler.baseFilename, 'a', encoding='utf-8')
            self.handler.emit(record)
        self.handler.close()

# Function to set up logging
def setup_logging(log_file='./runtime.log'):
    global _logging_setup_done
    if _logging_setup_done:
        return  # Prevent re-initialization if already set up

    handler = ConcurrentRotatingFileHandler(log_file, mode='a', encoding='utf-8', backupCount=5)
    handler.setFormatter(logging.Formatter('%(asctime)s:%(levelname)s:%(name)s: %(message)s'))

    queue_handler = QueueHandler(log_queue)
    queue_listener = QueueListener(log_queue, handler)

    logger = logging.getLogger()
    logger.setLevel(logging.DEBUG)
    logger.addHandler(queue_handler)
    logger.propagate = False

    # Ensure that the logging listener stops when the application exits
    import atexit
    atexit.register(log_queue.put, None)

    # Wait for all logs to be processed before exiting
    def cleanup():
        log_queue.put(None)
        queue_listener.join()

    atexit.register(cleanup)
    
    _logging_setup_done = True  # Mark setup as done