# watcher.py

import time
import traceback

while True:
    try:
        import main
        main.start_bot()
    except Exception as e:
        print("Bot crashed:")
        traceback.print_exc()
        print("Restarting in 5 seconds...")
        time.sleep(5)